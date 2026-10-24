package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUIPreferencesPersistAcrossHandlersAndRejectVPNFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ui", "preferences.json")
	for _, language := range []string{"en", "zh", "ru", "system"} {
		response := httptest.NewRecorder()
		newUIPreferencesHandler(path).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/ui/preferences", strings.NewReader(`{"language":"`+language+`"}`)))
		if response.Code != http.StatusOK || readUIPreferences(path).Language != language {
			t.Fatalf("save/restart language=%s: %d %s", language, response.Code, response.Body.String())
		}
		script := httptest.NewRecorder()
		uiLanguageScript(path).ServeHTTP(script, httptest.NewRequest(http.MethodGet, "/ui-language.js", nil))
		if !strings.Contains(script.Body.String(), `"`+language+`"`) {
			t.Fatalf("wrong bootstrap preference: %s", script.Body.String())
		}
	}
	original, _ := os.ReadFile(path)
	for _, body := range []string{`{"language":"de"}`, `{"language":"en","server":"evil"}`, `{"language":"en"}{}`, `{"language":"` + strings.Repeat("x", 5000) + `"}`} {
		response := httptest.NewRecorder()
		newUIPreferencesHandler(path).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/ui/preferences", strings.NewReader(body)))
		after, _ := os.ReadFile(path)
		if response.Code != http.StatusBadRequest || string(after) != string(original) {
			t.Fatalf("invalid preference changed stored state: status=%d", response.Code)
		}
	}
}
