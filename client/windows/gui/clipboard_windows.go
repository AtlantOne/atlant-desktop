package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	clipboardUser32   = windows.NewLazySystemDLL("user32.dll")
	clipboardKernel32 = windows.NewLazySystemDLL("kernel32.dll")
	openClipboard     = clipboardUser32.NewProc("OpenClipboard")
	closeClipboard    = clipboardUser32.NewProc("CloseClipboard")
	getClipboardData  = clipboardUser32.NewProc("GetClipboardData")
	globalLock        = clipboardKernel32.NewProc("GlobalLock")
	globalUnlock      = clipboardKernel32.NewProc("GlobalUnlock")
	globalSize        = clipboardKernel32.NewProc("GlobalSize")
)

func decodeClipboardText(units []uint16) (string, error) {
	for i, unit := range units {
		if unit == 0 {
			return string(utf16.Decode(units[:i])), nil
		}
	}
	return "", fmt.Errorf("clipboard text is not terminated")
}

// Clipboard ownership is thread-bound. Copy the data before releasing its lock.
func readWindowsClipboard() (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	opened := false
	for attempt := 0; attempt < 5; attempt++ {
		if ok, _, _ := openClipboard.Call(0); ok != 0 {
			opened = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !opened {
		return "", fmt.Errorf("clipboard is busy; try again")
	}
	defer closeClipboard.Call()
	const cfUnicodeText = 13
	handle, _, _ := getClipboardData.Call(cfUnicodeText)
	if handle == 0 {
		return "", nil
	}
	size, _, _ := globalSize.Call(handle)
	if size == 0 || size%2 != 0 || size > maxSubscriptionBytes {
		return "", fmt.Errorf("clipboard text is invalid or too large")
	}
	pointer, _, _ := globalLock.Call(handle)
	if pointer == 0 {
		return "", fmt.Errorf("clipboard text is unavailable")
	}
	defer globalUnlock.Call(handle)
	return decodeClipboardText(unsafe.Slice((*uint16)(unsafe.Pointer(pointer)), int(size/2)))
}

// Mounted behind newGUIHandler: authenticated same-origin POST only. Never log
// clipboard contents or accept a caller-supplied Windows clipboard format.
func clipboardTextHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	text, err := readWindowsClipboard()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"text": text})
}
