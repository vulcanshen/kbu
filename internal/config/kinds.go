package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// KindData is what the user set on each kind, keyed by its KubectlName
// (e.g. "pod" / "namespace" / "configmap"): its pin and its sort. The user
// writes it kind by kind, so it is data, not a setting (tdp E3: a list the
// user writes is data, even when it holds how the app works): kinds.yaml in
// DataDir(). Kinds that are not known now (a CRD uninstalled) stay in it
// but are dropped from the sidebar, so a re-install of the CRD silently
// restores the user's pin / sort.
type KindData map[string]ResourceKindConfigEntry

// KindDataPath is the pins' and sorts' file: kinds.yaml in DataDir(), which
// $KBU__DATA moves.
func KindDataPath() string {
	return filepath.Join(DataDir(), "kinds.yaml")
}

// LoadKindData reads the pins and sorts from KindDataPath(). A missing file
// is none yet, not an error.
func LoadKindData() (KindData, error) {
	return LoadKindDataFrom(KindDataPath())
}

// LoadKindDataFrom reads the pins and sorts at path.
func LoadKindDataFrom(path string) (KindData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var k KindData
	if err := yaml.Unmarshal(data, &k); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return k, nil
}

// Save writes the pins and sorts to KindDataPath().
func (k KindData) Save() error {
	return k.SaveTo(KindDataPath())
}

// SaveTo writes the pins and sorts to path, atomically like Config.SaveTo:
// a crash mid-write must not leave a half-written file.
func (k KindData) SaveTo(path string) error {
	data, err := yaml.Marshal(k)
	if err != nil {
		return fmt.Errorf("marshalling the pins and sorts: %w", err)
	}
	return writeFileAtomic(path, data)
}

// MoveKindData moves the pins and sorts an older kbu kept in config.yaml
// (resource_kind_config) to kinds.yaml in the data directory, at the first
// start (tdp E3: a new place). The rest of config.yaml stays as it was, its
// comments too. When kinds.yaml is there already, they stay in config.yaml
// for the user to merge; when they cannot be moved, kbu keeps using them
// in config.yaml (KindDataInConfig). A config.yaml kbu cannot read is left
// to the question about it.
func MoveKindData() []Moved {
	return moveKindData(ConfigPath(), KindDataPath())
}

func moveKindData(from, to string) []Moved {
	data, err := os.ReadFile(from)
	if err != nil {
		return nil
	}
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	root := doc.Content[0]
	at := -1
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "resource_kind_config" {
			at = i
		}
	}
	if at < 0 {
		return nil
	}
	var k KindData
	if root.Content[at+1].Decode(&k) != nil || len(k) == 0 {
		return nil
	}
	mv := Moved{What: "pins and sorts", From: from, To: to}
	if exists(to) {
		mv.Err = ErrThereAlready
		return []Moved{mv}
	}
	if err := k.SaveTo(to); err != nil {
		mv.Err = err
		return []Moved{mv}
	}
	root.Content = append(root.Content[:at], root.Content[at+2:]...)
	out, err := yaml.Marshal(&doc)
	if err == nil {
		err = writeFileAtomic(from, out)
	}
	if err != nil {
		// Not out of config.yaml: they stay there only, and kbu keeps using them.
		_ = os.Remove(to)
		mv.Err = err
	}
	return []Moved{mv}
}

// KindDataInConfig reports whether the pins and sorts are still in
// config.yaml with no kinds.yaml: an older kbu's, which could not be moved.
// kbu then keeps reading and writing them there (tdp E3).
func KindDataInConfig(c *Config) bool {
	return len(c.ResourceKindConfig) > 0 && !exists(KindDataPath())
}

// writeFileAtomic writes data to path through a tempfile in the same
// directory and a rename, creating the directory when it is not there.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("creating a tempfile in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing tempfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing tempfile: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming tempfile to %s: %w", path, err)
	}
	return nil
}

// GetPinned returns the pin state for kind, or nil when not pinned.
func (k KindData) GetPinned(kind string) *PinnedConfig {
	entry, ok := k[kind]
	if !ok {
		return nil
	}
	return entry.Pinned
}

// IsPinned is the bool convenience wrapper.
func (k KindData) IsPinned(kind string) bool {
	return k.GetPinned(kind) != nil
}

// SetPinned upserts the pin entry for kind. Caller chooses Order —
// typically via NextPinOrder() when appending, or any int when the
// user manually moves a pin around. Pre-existing pin is overwritten.
func (k *KindData) SetPinned(kind string, order int) {
	if *k == nil {
		*k = make(KindData)
	}
	entry := (*k)[kind]
	entry.Pinned = &PinnedConfig{Order: order}
	(*k)[kind] = entry
}

// UnsetPinned drops the pin state for kind. When the entry has no
// other active config left (IsEmpty), the entire map entry is removed
// so the YAML stays tidy. Without this housekeeping, unpinning the
// only thing on a kind would leave `kind: {}` in the file.
func (k *KindData) UnsetPinned(kind string) {
	entry, ok := (*k)[kind]
	if !ok {
		return
	}
	entry.Pinned = nil
	k.put(kind, entry)
}

// PinnedOrdered returns kinds in display order — ascending Pinned.Order.
// Empty slice when no pins exist. Stable across calls.
func (k KindData) PinnedOrdered() []string {
	type kv struct {
		kind  string
		order int
	}
	pairs := make([]kv, 0, len(k))
	for kind, entry := range k {
		if entry.Pinned != nil {
			pairs = append(pairs, kv{kind, entry.Pinned.Order})
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		return pairs[i].order < pairs[j].order
	})
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.kind
	}
	return out
}

// GetSort returns the full sort chain for kind — nil/empty when no
// sort is set. The returned slice is shared with the map, callers MUST
// treat it as read-only.
func (k KindData) GetSort(kind string) SortChain {
	return k[kind].Sort
}

// SetSort upserts a tier into the kind's chain. If `column` is
// already in the chain, its direction is updated in place — priority
// is preserved (no reorder). Otherwise the tier is appended as the
// lowest-priority entry. Empty direction is rejected silently to
// keep the chain well-formed.
func (k *KindData) SetSort(kind, column, direction string) {
	if column == "" || direction == "" {
		return
	}
	if *k == nil {
		*k = make(KindData)
	}
	entry := (*k)[kind]
	if idx := entry.Sort.IndexOf(column); idx >= 0 {
		entry.Sort[idx].Direction = direction
	} else {
		entry.Sort = append(entry.Sort, SortConfig{Column: column, Direction: direction})
	}
	(*k)[kind] = entry
}

// UnsetSortColumn removes a single tier from the kind's chain.
// Other tiers shift up to fill the gap (priority N+1 becomes N).
// When the chain becomes empty AND the entry has no other active
// config, the entire map entry is dropped — same housekeeping as
// UnsetPinned.
func (k *KindData) UnsetSortColumn(kind, column string) {
	entry, ok := (*k)[kind]
	if !ok {
		return
	}
	idx := entry.Sort.IndexOf(column)
	if idx < 0 {
		return
	}
	entry.Sort = append(entry.Sort[:idx], entry.Sort[idx+1:]...)
	if len(entry.Sort) == 0 {
		entry.Sort = nil
	}
	k.put(kind, entry)
}

// ResetSort clears the entire sort chain for kind. Same housekeeping
// as UnsetSortColumn (drops the map entry when nothing else lives
// there). Backs the "Reset" shortcut in the column picker.
func (k *KindData) ResetSort(kind string) {
	entry, ok := (*k)[kind]
	if !ok {
		return
	}
	entry.Sort = nil
	k.put(kind, entry)
}

// put stores entry for kind, or drops the kind when nothing is left on it.
func (k *KindData) put(kind string, entry ResourceKindConfigEntry) {
	if entry.IsEmpty() {
		delete(*k, kind)
	} else {
		(*k)[kind] = entry
	}
}

// NextPinOrder returns the next sparse Order to use when appending a
// new pin (max existing + 10). Sparse increment lets the user wedge a
// kind between two existing pins by manually editing YAML without
// having to renumber the rest.
func (k KindData) NextPinOrder() int {
	max := 0
	for _, entry := range k {
		if entry.Pinned != nil && entry.Pinned.Order > max {
			max = entry.Pinned.Order
		}
	}
	return max + 10
}
