package basicauth

import (
	"log/slog"

	"github.com/fsnotify/fsnotify"
)

// startWatcher listens for file-system events on b.filePath and atomically
// replaces the credential map on each Write or Create event.
// On a load error the existing credential map is retained unchanged.
// This function is intended to run in its own goroutine.
func startWatcher(b *BasicAuthProvider) {
	for {
		select {
		case event, ok := <-b.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				newCreds, err := loadFile(b.filePath)
				if err != nil {
					slog.Error("basicauth: failed to reload credentials, retaining last valid set",
						"file", b.filePath, "error", err)
					continue
				}
				b.creds.Store(newCreds)
				slog.Info("basicauth: reloaded credentials", "count", len(*newCreds), "file", b.filePath)
			}

		case err, ok := <-b.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("basicauth: file watcher error", "file", b.filePath, "error", err)
		}
	}
}
