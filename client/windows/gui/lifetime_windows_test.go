package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLifetimeHTTPDisconnect(t *testing.T) {
	shutdown := make(chan struct{}, 1)
	l := newUILifetime(time.Hour, 20*time.Millisecond, func() { shutdown <- struct{}{} })
	server := httptest.NewServer(l)
	defer server.Close()
	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal("not a lifetime stream")
	}
	if _, err := bufio.NewReader(response.Body).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	select {
	case <-shutdown:
	case <-time.After(2 * time.Second):
		t.Fatal("closed HTTP stream left backend alive")
	}
}

func TestGUIInstanceExclusionAndRelease(t *testing.T) {
	name := fmt.Sprintf(`Local\AtlantGUI-Test-%d`, time.Now().UnixNano())
	release, err := acquireGUIInstance(name)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := acquireGUIInstance(name); err == nil {
		other()
		release()
		t.Fatal("duplicate allowed")
	}
	release()
	release, err = acquireGUIInstance(name)
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestWindowCloseAndReload(t *testing.T) {
	shutdown := make(chan struct{}, 4)
	l := newUILifetime(time.Hour, 80*time.Millisecond, func() { shutdown <- struct{}{} })
	first := l.attach()
	first()
	first() // Closing the same connection twice must not change the count.
	second := l.attach()
	select {
	case <-shutdown:
		t.Fatal("reload stopped the application")
	case <-time.After(160 * time.Millisecond):
	}
	second()
	select {
	case <-shutdown:
	case <-time.After(2 * time.Second):
		t.Fatal("window closure left the backend running")
	}
}

func TestOpenWindowNeedsNoHeartbeatAndStartupExpires(t *testing.T) {
	shutdown := make(chan struct{}, 4)
	l := newUILifetime(80*time.Millisecond, 20*time.Millisecond, func() { shutdown <- struct{}{} })
	detach := l.attach()
	select {
	case <-shutdown:
		t.Fatal("open window needs polling")
	case <-time.After(160 * time.Millisecond):
	}
	detach()
	select {
	case <-shutdown:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown missing")
	}
	newUILifetime(20*time.Millisecond, time.Second, func() { shutdown <- struct{}{} })
	select {
	case <-shutdown:
	case <-time.After(2 * time.Second):
		t.Fatal("failed browser startup left backend running")
	}
}
