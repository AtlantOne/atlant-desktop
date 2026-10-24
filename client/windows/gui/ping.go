package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"
)

// This measures TCP connect latency, not authenticated VPN readiness or ICMP.
func newPingHandler(load func() *AppConfig, dial func(context.Context, string, string) (net.Conn, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		srv := r.URL.Query().Get("server")
		if srv == "" {
			cfg := load()
			srv = cfg.Server
			if srv == "" && cfg.ActiveServerID != "" {
				for _, p := range cfg.Servers {
					if p.ID == cfg.ActiveServerID {
						srv = p.Server
						break
					}
				}
			}
		}
		fail := func(code int, kind, message string) {
			w.WriteHeader(code)
			_ = json.NewEncoder(w).Encode(map[string]any{"rtt": -1, "kind": "tcp_connect", "error_code": kind, "error": message})
		}
		if srv == "" {
			fail(http.StatusBadRequest, "no_server", "Сначала выберите сервер")
			return
		}
		if host, port, err := net.SplitHostPort(srv); err != nil || host == "" || port == "" {
			fail(http.StatusBadRequest, "invalid_address", "Неверный адрес сервера: нужен IP или домен и порт")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		start := time.Now()
		conn, err := dial(ctx, "tcp", srv)
		elapsed := time.Since(start)
		if err != nil {
			fail(http.StatusOK, "dial_failed", err.Error())
			return
		}
		_ = conn.Close()
		_ = json.NewEncoder(w).Encode(map[string]any{"rtt": elapsed.Milliseconds(), "kind": "tcp_connect"})
	}
}
