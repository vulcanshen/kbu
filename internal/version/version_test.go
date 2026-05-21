package version

import "testing"

func TestDisplay(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"dev build", "dev", "dev"},
		{"tagged release", "1.0.9", "v1.0.9"},
		{"semver pre-release", "1.0.10-rc.1", "v1.0.10-rc.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := Version
			Version = tt.version
			defer func() { Version = orig }()
			if got := Display(); got != tt.want {
				t.Errorf("Display() = %q, want %q", got, tt.want)
			}
		})
	}
}
