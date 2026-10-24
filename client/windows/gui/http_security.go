package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const maxSubscriptionBytes = 4 << 20

var guiConfigMutationMu sync.Mutex

// The GUI runs elevated. Loopback binding alone does not prevent browser CSRF
// or DNS rebinding. No remote origin or unauthenticated API request is allowed.
func newGUIBootstrapToken() (string, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(random[:]), nil
}

func newGUIHandler(host string, next http.Handler, bootstrapToken string) (http.Handler, error) {
	if len(bootstrapToken) != 64 {
		return nil, fmt.Errorf("invalid GUI bootstrap token")
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	secret := hex.EncodeToString(random[:])
	var mutationMu sync.Mutex
	var bootstrapMu sync.Mutex
	cookieName := "atlant-session-" + strings.ReplaceAll(host, ":", "-")
	readOnly := map[string]bool{"/api/status": true, "/api/config": true, "/api/ping": true, "/api/apps/running": true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		origin := r.Header.Get("Origin")
		if r.Host != host || (origin != "" && origin != "http://"+host) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		api := strings.HasPrefix(r.URL.Path, "/api/")
		if api {
			cookie, err := r.Cookie(cookieName)
			if err != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(secret)) != 1 {
				http.Error(w, "GUI session required", http.StatusUnauthorized)
				return
			}
			if r.Method != http.MethodPost && !(readOnly[r.URL.Path] && r.Method == http.MethodGet) {
				w.Header().Set("Allow", "POST")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
		} else {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if r.URL.Path == "/" || r.URL.Path == "/index.html" {
				if token := r.URL.Query().Get("bootstrap"); token != "" {
					bootstrapMu.Lock()
					valid := bootstrapToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(bootstrapToken)) == 1
					if valid {
						bootstrapToken = ""
					}
					bootstrapMu.Unlock()
					if !valid {
						http.Error(w, "invalid or consumed GUI bootstrap", http.StatusUnauthorized)
						return
					}
					http.SetCookie(w, &http.Cookie{Name: cookieName, Value: secret, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
					http.Redirect(w, r, "/", http.StatusSeeOther)
					return
				}
			}
		}
		if api && r.Method == http.MethodPost {
			mutationMu.Lock()
			defer mutationMu.Unlock()
		}
		if r.Method == http.MethodPost && (r.URL.Path == "/api/config" || strings.HasPrefix(r.URL.Path, "/api/servers/") || strings.HasPrefix(r.URL.Path, "/api/subscription/")) {
			guiConfigMutationMu.Lock()
			defer guiConfigMutationMu.Unlock()
		}
		next.ServeHTTP(w, r)
	}), nil
}

func readSubscriptionResponse(resp *http.Response) ([]byte, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscription HTTP status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSubscriptionBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSubscriptionBytes {
		return nil, fmt.Errorf("subscription exceeds %d bytes", maxSubscriptionBytes)
	}
	return data, nil
}

// Replace only after the whole configuration is on disk. Readers never see a
// partially truncated JSON file. This does not replace Windows ACL/DPAPI storage.
func saveGUIConfigFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".atlant-config-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
