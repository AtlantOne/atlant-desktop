package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGUIRejectsCSRFRebindingAndMutatingGET(t *testing.T) {
	host := "127.0.0.1:4567"
	called := 0
	token := strings.Repeat("a", 64)
	h, err := newGUIHandler(host, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called++; w.WriteHeader(204) }), token)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := httptest.NewRecorder()
	public := httptest.NewRecorder()
	h.ServeHTTP(public, httptest.NewRequest("GET", "http://"+host+"/", nil))
	if len(public.Result().Cookies()) != 0 {
		t.Fatal("anonymous client obtained privileged cookie")
	}
	h.ServeHTTP(bootstrap, httptest.NewRequest("GET", "http://"+host+"/?bootstrap="+token, nil))
	cookies := bootstrap.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe bootstrap cookie")
	}
	replay := httptest.NewRecorder()
	h.ServeHTTP(replay, httptest.NewRequest("GET", "http://"+host+"/?bootstrap="+token, nil))
	if replay.Code != 401 || len(replay.Result().Cookies()) != 0 {
		t.Fatal("bootstrap token replayed")
	}
	for _, test := range []struct {
		method, path, origin, site, requestHost string
		cookie                                  bool
		status                                  int
	}{
		{"POST", "/api/disconnect", "", "", host, false, 401},
		{"GET", "/api/config", "", "", host, false, 401},
		{"GET", "/api/ui/lifetime", "", "", host, false, 401},
		{"GET", "/api/ui/lifetime", "https://evil.example", "", host, true, 403},
		{"GET", "/api/ui/lifetime", "http://" + host, "same-origin", host, true, 204},
		{"POST", "/api/clipboard/text", "", "", host, false, 401},
		{"GET", "/api/clipboard/text", "", "", host, true, 405},
		{"POST", "/api/clipboard/text", "https://evil.example", "", host, true, 403},
		{"POST", "/api/clipboard/text", "http://" + host, "same-origin", host, true, 204},
		{"POST", "/api/disconnect", "https://evil.example", "", host, true, 403},
		{"POST", "/api/disconnect", "null", "", host, true, 403},
		{"POST", "/api/disconnect", "", "cross-site", host, true, 403},
		{"POST", "/api/disconnect", "", "", "evil.example:4567", true, 403},
		{"GET", "/api/disconnect", "", "", host, true, 405},
		{"POST", "/api/disconnect", "http://" + host, "same-origin", host, true, 204},
		{"GET", "/api/config", "", "same-origin", host, true, 204},
	} {
		t.Run(test.method+test.path+test.origin+test.site+test.requestHost, func(t *testing.T) {
			r := httptest.NewRequest(test.method, "http://"+host+test.path, nil)
			r.Host = test.requestHost
			r.Header.Set("Origin", test.origin)
			r.Header.Set("Sec-Fetch-Site", test.site)
			if test.cookie {
				r.AddCookie(cookies[0])
			}
			w := httptest.NewRecorder()
			before := called
			h.ServeHTTP(w, r)
			if w.Code != test.status {
				t.Fatalf("status=%d want=%d", w.Code, test.status)
			}
			if test.status != 204 && called != before {
				t.Fatal("blocked request reached privileged handler")
			}
		})
	}
}

func TestSubscriptionResponseBoundedAndStatusChecked(t *testing.T) {
	for _, test := range []struct {
		status, size int
		fail         bool
	}{{200, 128, false}, {200, maxSubscriptionBytes, false}, {200, maxSubscriptionBytes + 1, true}, {500, 1, true}} {
		r := &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", test.size)))}
		_, err := readSubscriptionResponse(r)
		if (err != nil) != test.fail {
			t.Fatalf("status=%d size=%d err=%v", test.status, test.size, err)
		}
	}
}

func TestGUIConfigReplaceIsComplete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	for _, data := range []string{`{"value":"old"}`, `{"value":"new"}`} {
		if err := saveGUIConfigFile(path, []byte(data)); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != data {
			t.Fatalf("config=%q err=%v", got, err)
		}
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".atlant-config-*"))
	if err != nil || len(files) != 0 {
		t.Fatal("temporary config leaked")
	}
}
