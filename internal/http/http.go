package http

import (
	"fmt"
	"log/slog"
	"net/http"
)

// Start starts the healthcheck server on the given port.
func Start(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	addr := fmt.Sprintf(":%d", port)
	slog.Info("starting healthcheck server", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("healthcheck server failed", "err", err)
	}
}
