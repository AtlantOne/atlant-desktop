package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedCatalogProtectedAndListRedacted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "managed.json")
	old := adminCatalogFile
	adminCatalogFile = func() (string, error) { return path, nil }
	defer func() { adminCatalogFile = old }()
	call := func(op, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/admin/"+op, strings.NewReader(body))
		w := httptest.NewRecorder()
		adminHandler(w, r)
		return w
	}
	save := call("save", `{"name":"Test VDS","request":{"host":"example.com","username":"root","password":"private-SSH-password","fingerprint":"SHA256:example","port":22}}`)
	if save.Code != 200 {
		t.Fatal(save.Body.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private-SSH-password") || strings.Contains(string(data), "example.com") {
		t.Fatal("catalog stored in plaintext")
	}
	list := call("list", `{}`)
	if list.Code != 200 || strings.Contains(list.Body.String(), "private-SSH-password") {
		t.Fatal("list leaked credentials")
	}
	stored, err := readAdminCatalog()
	if err != nil || len(stored) != 1 || stored[0].Request.Password != "private-SSH-password" {
		t.Fatal("protected roundtrip failed", err)
	}
	var item struct {
		ID string `json:"id"`
	}
	json.Unmarshal(save.Body.Bytes(), &item)
	remove := call("remove", `{"id":"`+item.ID+`"}`)
	if remove.Code != 200 {
		t.Fatal(remove.Body.String())
	}
}

func TestManagedSessionOpenRejectsUnknownJob(t *testing.T) {
	w := httptest.NewRecorder()
	adminHandler(w, httptest.NewRequest("POST", "/api/admin/open", strings.NewReader(`{"job":"missing"}`)))
	if w.Code == 200 {
		t.Fatal("unknown management session accepted")
	}
}
