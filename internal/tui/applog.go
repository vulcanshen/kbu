package tui

import (
	"unicode"
	"unicode/utf8"

	terminu "github.com/vulcanshen/terminu-go"
)

// appLog writes to the lib's App Log (! on a panel; tdp note.7):
// what kbu does and what it does in the background. Toasts and error popups
// go in by themselves (tdp F5), so what one says is not logged again. Each
// entry starts with a capital, as a toast does (tdp note-78).
type appLog struct{ app *terminu.App }

func (l appLog) info(text string)    { l.app.Log(terminu.Info, text) }
func (l appLog) warn(text string)    { l.app.Log(terminu.Warning, text) }
func (l appLog) err(text string)     { l.app.Log(terminu.Error, text) }
func (l appLog) success(text string) { l.app.Log(terminu.Success, text) }

// capital starts text with a capital, for a toast or an App Log entry made of
// words kbu wrote in lower case: a Go error, a verb kept for a switch.
func capital(text string) string {
	r, n := utf8.DecodeRuneInString(text)
	if n == 0 {
		return text
	}
	return string(unicode.ToUpper(r)) + text[n:]
}
