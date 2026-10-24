package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http/httptest"
	"testing"
)

func TestPingReturnsJSONForUnselectedAndInvalidServer(t *testing.T) {
	for _, server := range []string{"", "missing-port"} {
		called := false
		h := newPingHandler(func() *AppConfig { return &AppConfig{Server: server} }, func(context.Context, string, string) (net.Conn, error) {
			called = true
			return nil, errors.New("unexpected dial")
		})
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest("GET", "/api/ping", nil))
		var result map[string]any
		if json.Unmarshal(w.Body.Bytes(), &result) != nil || w.Code != 400 || result["rtt"] != float64(-1) || result["error_code"] == nil || called {
			t.Fatalf("invalid diagnostic: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestPingResolvesSelectedCatalogProfileAndPreservesDialError(t *testing.T) {
	cfg := &AppConfig{ActiveServerID: "picked", Servers: []ServerProfile{{ID: "other", Server: "wrong:1"}, {ID: "picked", Server: "127.0.0.1:1234"}}}
	h := newPingHandler(func() *AppConfig { return cfg }, func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr != "127.0.0.1:1234" || network != "tcp" {
			t.Fatal("wrong selected address")
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("unbounded probe")
		}
		return nil, errors.New("fixture connect timeout")
	})
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest("GET", "/api/ping", nil))
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["error_code"] != "dial_failed" || result["kind"] != "tcp_connect" || result["error"] != "fixture connect timeout" {
		t.Fatal(result)
	}
}

func TestPingQueryUsesRequestedNode(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	h := newPingHandler(func() *AppConfig { t.Fatal("unexpected config access"); return nil }, func(_ context.Context, _, addr string) (net.Conn, error) {
		if addr != "127.0.0.1:2345" {
			t.Fatal(addr)
		}
		return a, nil
	})
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest("GET", "/api/ping?server=127.0.0.1:2345", nil))
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || result["kind"] != "tcp_connect" || result["rtt"].(float64) < 0 {
		t.Fatal(result)
	}
}
