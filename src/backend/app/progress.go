package app

import wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

type progressReporter struct {
	app *App
}

func (p progressReporter) UpdateProgress(curr, total int, message string) {
	p.app.updateProgress(curr, total, message)
}

func (a *App) updateProgress(curr, total int, message string) {
	progressMessage := ProgressUpdate{Curr: curr, Total: total, Message: message}
	a.sugarLogger.Debugf("%v (%v/%v)", message, curr, total)
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "updateProgress", progressMessage)
	}
}
