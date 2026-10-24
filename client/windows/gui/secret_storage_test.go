package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDPAPIEncryptedRoundTripAndRandomization(t *testing.T) {
	plain := []byte(`{"pbk":"test-secret-not-a-real-key","subscription":{"url":"https://example.com/private-token"}}`)
	a, err := protectConfig(plain, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := protectConfig(plain, false)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) || bytes.Contains(a, []byte("private-token")) {
		t.Fatal("ciphertext repeated or cleartext exposed")
	}
	got, err := protectConfig(a, true)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatal("round trip failed", err)
	}
	a[len(a)-1] ^= 1
	if _, err := protectConfig(a, true); err == nil {
		t.Fatal("tamper accepted")
	}
}

func TestProtectedConfigMigratesWithoutPlaintextAndRefusesCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	plain := []byte(`{"pbk":"test-secret-not-a-real-key"}`)
	if err := os.WriteFile(path, plain, 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveProtectedConfig(path, plain); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(path)
	if err != nil || bytes.Contains(stored, []byte("test-secret")) {
		t.Fatal("plaintext remains")
	}
	got, legacy, err := decodeStoredConfig(stored)
	if err != nil || legacy || !bytes.Equal(got, plain) {
		t.Fatal("decode failed", err)
	}
	var envelope protectedConfig
	if err := json.Unmarshal(stored, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.Data[0] ^= 1
	broken, _ := json.Marshal(envelope)
	if err := os.WriteFile(path, broken, 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveProtectedConfig(path, []byte(`{}`)); err == nil {
		t.Fatal("corrupt config overwritten")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, broken) {
		t.Fatal("original damaged config lost")
	}
}
