//go:build pprof

package application

import (
	"log/slog"
	"net/http"
	_ "net/http/pprof" // registers pprof handlers on http.DefaultServeMux
)

const pprofAddr = ":6060"

func startPprof() {
	slog.Info("pprof enabled", "addr", pprofAddr)
	go func() {
		if err := http.ListenAndServe(pprofAddr, nil); err != nil {
			slog.Error("pprof server stopped", "err", err)
		}
	}()
}
