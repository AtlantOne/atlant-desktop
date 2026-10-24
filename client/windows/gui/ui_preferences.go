package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type uiPreferences struct {
	Language string `json:"language"`
}

func validUILanguage(language string) bool {
	return language == "system" || language == "en" || language == "ru" || language == "zh"
}

func readUIPreferences(path string) uiPreferences {
	prefs := uiPreferences{Language: "system"}
	file, err := os.Open(path)
	if err != nil {
		return prefs
	}
	defer file.Close()
	var stored uiPreferences
	if json.NewDecoder(io.LimitReader(file, 4096)).Decode(&stored) == nil && validUILanguage(stored.Language) {
		return stored
	}
	return prefs
}

// Separate from the VPN configuration: choosing a language never reconnects.
func newUIPreferencesHandler(path string) http.Handler {
	var mu sync.Mutex
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(readUIPreferences(path))
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var prefs uiPreferences
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&prefs) != nil || !validUILanguage(prefs.Language) || decoder.Decode(new(any)) != io.EOF {
			http.Error(w, "Invalid UI preferences", http.StatusBadRequest)
			return
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			http.Error(w, "Cannot save UI preferences", http.StatusInternalServerError)
			return
		}
		file, err := os.CreateTemp(filepath.Dir(path), ".ui-preferences-*")
		if err != nil {
			http.Error(w, "Cannot save UI preferences", http.StatusInternalServerError)
			return
		}
		temporary := file.Name()
		defer os.Remove(temporary)
		err = json.NewEncoder(file).Encode(prefs)
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(temporary, path)
		}
		if err != nil {
			http.Error(w, "Cannot save UI preferences", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(prefs)
	})
}

func uiLanguageScript(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		encoded, _ := json.Marshal(readUIPreferences(path).Language)
		_, _ = w.Write(append(append([]byte("window.atlantUiLanguage = "), encoded...), ';'))
	}
}
