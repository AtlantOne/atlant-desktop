package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

func acquireGUIInstance(name string) (func(), error) {
	value, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateMutex(nil, false, value)
	if err != nil {
		if handle != 0 {
			windows.CloseHandle(handle)
		}
		return nil, fmt.Errorf("Atlant is already running or its instance lock is unavailable: %w", err)
	}
	return func() { windows.CloseHandle(handle) }, nil
}

// A persistent connection survives minimization; reconnect grace allows reloads.
type uiLifetime struct {
	mu          sync.Mutex
	connections int
	timer       *time.Timer
	generation  uint64
	grace       time.Duration
	shutdown    func()
}

func newUILifetime(startup, grace time.Duration, shutdown func()) *uiLifetime {
	l := &uiLifetime{grace: grace, shutdown: shutdown}
	l.schedule(startup)
	return l
}

func (l *uiLifetime) schedule(delay time.Duration) {
	l.generation++
	generation := l.generation
	l.timer = time.AfterFunc(delay, func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.connections == 0 && l.generation == generation {
			l.shutdown()
		}
	})
}

func (l *uiLifetime) attach() func() {
	l.mu.Lock()
	l.generation++
	if l.timer != nil {
		l.timer.Stop()
	}
	l.connections++
	l.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.connections--
			if l.connections == 0 {
				l.schedule(l.grace)
			}
		})
	}
}

func (l *uiLifetime) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		http.Error(w, "lifetime streaming unavailable", http.StatusInternalServerError)
		return
	}
	detach := l.attach()
	defer detach()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	controller := http.NewResponseController(w)
	write := func() bool {
		if _, err := fmt.Fprint(w, ": alive\n\n"); err != nil {
			return false
		}
		return controller.Flush() == nil
	}
	if !write() {
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if !write() {
				return
			}
		}
	}
}
