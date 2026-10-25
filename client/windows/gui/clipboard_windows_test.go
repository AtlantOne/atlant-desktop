package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"unicode/utf16"
)

func TestWindowsClipboardRead(t *testing.T) {
	if os.Getenv("ATLANT_TEST_CLIPBOARD_READ") != "1" {
		t.Skip("explicit opt-in required to read the desktop clipboard")
	}
	if _, err := readWindowsClipboard(); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeClipboardText(t *testing.T) {
	for _, text := range []string{"", "atlant://example", "https://例子.测试/подписка🙂\r\nhttps://example.test/sub"} {
		units := append(utf16.Encode([]rune(text)), 0, 'x')
		got, err := decodeClipboardText(units)
		if err != nil || got != text {
			t.Fatalf("UTF-16 roundtrip failed: %v", err)
		}
	}
	if _, err := decodeClipboardText([]uint16{'x'}); err == nil {
		t.Fatal("unterminated clipboard accepted")
	}
}

func TestClipboardRejectsGET(t *testing.T) {
	w := httptest.NewRecorder()
	clipboardTextHandler(w, httptest.NewRequest(http.MethodGet, "/api/clipboard/text", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d", w.Code)
	}
}
