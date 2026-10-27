package main

import (
	"encoding/json"
	"os"
	"testing"
)

func firstSelectionProfile(id, address string) ServerProfile {
	return ServerProfile{
		ID: id, Name: "Test server", Server: address, SNI: "example.com",
		User: "test-user", PBK: "test-secret", ServerPubKey: "test-public-key",
		WireVersion: 7, Path: "/test", Transport: "reality",
	}
}

func assertSelectedProfile(t *testing.T, cfg *AppConfig, p ServerProfile) {
	t.Helper()
	if cfg.ActiveServerID != p.ID || cfg.Server != p.Server || cfg.SNI != p.SNI ||
		cfg.User != p.User || cfg.PBK != p.PBK || cfg.ServerPubKey != p.ServerPubKey ||
		cfg.WireVersion != p.WireVersion || cfg.Path != p.Path {
		t.Fatal("selected profile and connection parameters differ")
	}
}

func TestFirstProfileSelectionPersists(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := loadConfig()
	p := firstSelectionProfile("first", "first.example:443")
	cfg.Servers = append(cfg.Servers, p)
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	assertSelectedProfile(t, cfg, p)
	assertSelectedProfile(t, loadConfig(), p)
	second := firstSelectionProfile("second", "second.example:443")
	cfg.Servers = append(cfg.Servers, second)
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	assertSelectedProfile(t, loadConfig(), p)
}

func TestFirstSubscriptionProfileSelectedOnSave(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := loadConfig()
	upsertSubscription(cfg, Subscription{ID: "sub", URL: "https://example.com/sub"},
		[]ServerProfile{firstSelectionProfile("incoming", "first.example:443"), firstSelectionProfile("other", "second.example:443")})
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	assertSelectedProfile(t, loadConfig(), cfg.Servers[0])
}

func TestLoadRepairsPreviouslyUnselectedProfile(t *testing.T) {
	t.Chdir(t.TempDir())
	p := firstSelectionProfile("first", "first.example:443")
	data, err := json.Marshal(&AppConfig{Servers: []ServerProfile{p}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("atlant_config.json", data, 0600); err != nil {
		t.Fatal(err)
	}
	assertSelectedProfile(t, loadConfig(), p)
}

func TestFirstProfileSelectionPreservesExistingConnection(t *testing.T) {
	p := firstSelectionProfile("first", "first.example:443")
	for _, cfg := range []*AppConfig{
		{ActiveServerID: "chosen", Server: "chosen.example:443", Servers: []ServerProfile{p}},
		{Server: "legacy.example:443", Servers: []ServerProfile{p}},
	} {
		id, address := cfg.ActiveServerID, cfg.Server
		selectFirstProfileIfUnset(cfg)
		if cfg.ActiveServerID != id || cfg.Server != address {
			t.Fatal("adding a profile changed the existing connection")
		}
	}
}

func TestFirstProfileSelectionSkipsEmptyEntries(t *testing.T) {
	cfg := &AppConfig{Servers: []ServerProfile{{}, {ID: "empty"}}}
	selectFirstProfileIfUnset(cfg)
	if cfg.ActiveServerID != "" || cfg.Server != "" {
		t.Fatal("selected an empty profile")
	}
	p := firstSelectionProfile("valid", "first.example:443")
	cfg.Servers = append(cfg.Servers, p)
	selectFirstProfileIfUnset(cfg)
	assertSelectedProfile(t, cfg, p)
}
