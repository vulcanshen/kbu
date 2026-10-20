package tui

import terminu "github.com/vulcanshen/terminu-go"

// appLog writes to the lib's App Log (! on a panel; components/dialog/note):
// what kbu does and what it does in the background. Toasts and error popups
// go in by themselves (tdp F5), so what one says is not logged again.
type appLog struct{ app *terminu.App }

func (l appLog) info(text string)    { l.app.Log(terminu.Info, text) }
func (l appLog) warn(text string)    { l.app.Log(terminu.Warning, text) }
func (l appLog) err(text string)     { l.app.Log(terminu.Error, text) }
func (l appLog) success(text string) { l.app.Log(terminu.Success, text) }
