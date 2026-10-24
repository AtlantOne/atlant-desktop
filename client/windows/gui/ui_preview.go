//go:build uipreview

// Run explicitly: go run ui_preview.go http_security.go
// Mock-only GUI preview: no elevation, profiles, VPN, routes or registry access.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	var mu sync.Mutex
	connected := false
	config := map[string]any{"server": "198.51.100.20:443", "transport": "quic", "tun": true, "mtu": 1360, "dns": "1.1.1.1", "block_ipv6": true, "padding_enabled": true, "chaffing_enabled": true, "active_server_id": "demo", "servers": []any{map[string]any{"id": "demo", "name": "Тестовый профиль · Preview", "server": "198.51.100.20:443", "transport": "quic", "type": "atlant", "ping": 24}}, "subscription": map[string]any{}}
	mux := http.NewServeMux()
	preferences, err := os.MkdirTemp("", "atlant-ui-preview-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(preferences)
	mux.Handle("/api/ui/preferences", newUIPreferencesHandler(preferences+"/ui.json"))
	mux.HandleFunc("/ui-language.js", uiLanguageScript(preferences+"/ui.json"))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/config":
			if r.Method == "POST" {
				if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
					http.Error(w, "bad demo config", 400)
					return
				}
			}
			_ = json.NewEncoder(w).Encode(config)
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"connected": connected, "logs": "[PREVIEW] Макет интерфейса. VPN не запускается."})
		case "/api/connect":
			connected = true
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		case "/api/disconnect":
			connected = false
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		case "/api/ping":
			_ = json.NewEncoder(w).Encode(map[string]int{"rtt": 24})
		case "/api/apps/running":
			_ = json.NewEncoder(w).Encode(map[string]any{"apps": []string{}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "servers": config["servers"]})
		}
	})
	mux.Handle("/", http.FileServer(http.FS(os.DirFS("web"))))
	bootstrap, err := newGUIBootstrapToken()
	if err != nil {
		log.Fatal(err)
	}
	handler, err := newGUIHandler(listener.Addr().String(), mux, bootstrap)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("UI_PREVIEW=http://%s/?bootstrap=%s\n", listener.Addr(), bootstrap)
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.Serve(listener))
}
