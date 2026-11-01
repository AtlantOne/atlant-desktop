package main

import (
	"atlant.local/remoteadmin"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type adminServer struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Request        remoteadmin.Request `json:"request"`
	ActiveJob      string              `json:"active_job,omitempty"`
	HasCredentials bool                `json:"has_credentials,omitempty"`
}

var adminCatalogMu sync.Mutex
var adminServerJobs = map[string]string{}
var adminCatalogFile = adminCatalogPath

func adminCatalogPath() (string, error) {
	folder, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	folder = filepath.Join(folder, "AtlantVPN")
	if err = os.MkdirAll(folder, 0700); err != nil {
		return "", err
	}
	return filepath.Join(folder, "admin-servers.json"), nil
}
func readAdminCatalog() ([]adminServer, error) {
	path, err := adminCatalogFile()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []adminServer{}, nil
	}
	if err != nil {
		return nil, err
	}
	plain, legacy, err := decodeStoredConfig(data)
	if err != nil || legacy {
		return nil, errors.New("protected managed-server catalog unavailable")
	}
	var list []adminServer
	err = json.Unmarshal(plain, &list)
	return list, err
}
func writeAdminCatalog(list []adminServer) error {
	plain, err := json.Marshal(list)
	if err != nil {
		return err
	}
	sealed, err := protectConfig(plain, false)
	if err != nil {
		return err
	}
	data, err := json.Marshal(protectedConfig{Format: protectedConfigFormat, Data: sealed})
	if err != nil {
		return err
	}
	path, err := adminCatalogFile()
	if err != nil {
		return err
	}
	return saveGUIConfigFile(path, data)
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var input struct {
		ID        string              `json:"id"`
		Name      string              `json:"name"`
		Request   remoteadmin.Request `json:"request"`
		Remember  bool                `json:"remember"`
		Job       string              `json:"job"`
		Command   string              `json:"command"`
		Confirmed bool                `json:"confirmed"`
		Reveal    bool                `json:"reveal"`
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 128<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	operation := strings.TrimPrefix(r.URL.Path, "/api/admin/")
	respond := func(value any) { _ = json.NewEncoder(w).Encode(value) }
	fail := func(err error) { w.WriteHeader(400); respond(map[string]string{"error": err.Error()}) }
	if operation == "probe" {
		fingerprint, err := remoteadmin.Probe(input.Request.Host, input.Request.Port)
		if err != nil {
			fail(err)
			return
		}
		respond(map[string]string{"fingerprint": fingerprint})
		return
	}
	if operation == "status" {
		_, _ = io.WriteString(w, remoteadmin.Status(input.Job))
		return
	}
	if operation == "close" {
		remoteadmin.Close(input.Job)
		adminCatalogMu.Lock()
		for id, job := range adminServerJobs {
			if job == input.Job {
				delete(adminServerJobs, id)
			}
		}
		adminCatalogMu.Unlock()
		respond(map[string]bool{"ok": true})
		return
	}
	if operation == "command" {
		body, _ := json.Marshal(map[string]any{"command": input.Command, "confirmed": input.Confirmed, "reveal": input.Reveal})
		result, err := remoteadmin.Call(input.Job, string(body))
		if err != nil {
			fail(err)
			return
		}
		_, _ = io.WriteString(w, result)
		return
	}
	adminCatalogMu.Lock()
	defer adminCatalogMu.Unlock()
	list, err := readAdminCatalog()
	if err != nil {
		fail(err)
		return
	}
	index := -1
	for i, item := range list {
		if item.ID == input.ID {
			index = i
			break
		}
	}
	switch operation {
	case "list":
		for i := range list {
			list[i].ActiveJob = adminServerJobs[list[i].ID]
			list[i].HasCredentials = list[i].Request.Password != "" || list[i].Request.PrivateKey != ""
			list[i].Request.Password = ""
			list[i].Request.PrivateKey = ""
		}
		respond(list)
	case "save":
		if strings.TrimSpace(input.Name) == "" {
			input.Name = input.Request.Host
		}
		if input.Request.Host == "" {
			fail(errors.New("server name and SSH host required"))
			return
		}
		if index < 0 {
			if len(list) >= 32 {
				fail(errors.New("managed server limit reached"))
				return
			}
			var id [12]byte
			if _, err := rand.Read(id[:]); err != nil {
				fail(err)
				return
			}
			input.ID = hex.EncodeToString(id[:])
			list = append(list, adminServer{})
			index = len(list) - 1
		}
		request := input.Request
		request.Install = false
		old := list[index].Request
		if request.Password == "" && request.PrivateKey == "" && old.Host == request.Host && old.Port == request.Port && old.Username == request.Username && old.Fingerprint == request.Fingerprint {
			request.Password = old.Password
			request.PrivateKey = old.PrivateKey
		}
		if old.Host != request.Host || old.Port != request.Port || old.Username != request.Username || old.Fingerprint != request.Fingerprint {
			remoteadmin.Close(adminServerJobs[input.ID])
			delete(adminServerJobs, input.ID)
		}
		list[index] = adminServer{ID: input.ID, Name: input.Name, Request: request}
		if err := writeAdminCatalog(list); err != nil {
			fail(err)
			return
		}
		respond(map[string]string{"id": input.ID})
	case "remove":
		if index < 0 {
			fail(errors.New("server not found"))
			return
		}
		if id := adminServerJobs[input.ID]; id != "" {
			remoteadmin.Close(id)
			delete(adminServerJobs, input.ID)
		}
		list = append(list[:index], list[index+1:]...)
		if err := writeAdminCatalog(list); err != nil {
			fail(err)
			return
		}
		respond(map[string]bool{"ok": true})
	case "begin":
		if index >= 0 && !input.Request.Install && input.Request.Host == list[index].Request.Host && input.Request.Port == list[index].Request.Port && input.Request.Username == list[index].Request.Username && input.Request.Fingerprint == list[index].Request.Fingerprint {
			if existing := adminServerJobs[input.ID]; existing != "" {
				var state struct{ Phase string }
				_ = json.Unmarshal([]byte(remoteadmin.Status(existing)), &state)
				if state.Phase != "failed" && state.Phase != "closed" && state.Phase != "missing" && state.Phase != "stopped" {
					respond(map[string]string{"job": existing})
					return
				}
			}
		}
		request := input.Request
		if index >= 0 {
			saved := list[index].Request
			if request.Host == saved.Host && request.Port == saved.Port && request.Username == saved.Username && request.Fingerprint == saved.Fingerprint && request.Password == "" {
				request.Password = saved.Password
				request.PrivateKey = saved.PrivateKey
			}
		}
		data, _ := json.Marshal(request)
		if old := adminServerJobs[input.ID]; old != "" {
			remoteadmin.Close(old)
			delete(adminServerJobs, input.ID)
		}
		id, err := remoteadmin.Begin(string(data))
		if err != nil {
			fail(err)
			return
		}
		if index >= 0 {
			adminServerJobs[input.ID] = id
		}
		respond(map[string]string{"job": id})
	default:
		http.NotFound(w, r)
	}
}
