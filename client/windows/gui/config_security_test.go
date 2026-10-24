package main

import (
	"os"
	"testing"
)

func TestGUIStartsWithoutEmbeddedCredentials(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := loadConfig()
	if cfg.Server != "" || cfg.PBK != "" || cfg.User != "" || len(cfg.Servers) != 0 {
		t.Fatal("embedded profile/credentials in clean installation")
	}
	if cfg.LocalPort != "127.0.0.1:1080" {
		t.Fatalf("unsafe SOCKS default %q", cfg.LocalPort)
	}
	if cfg.WireVersion != 4 {
		t.Fatal("new client defaults to retired wire format")
	}
}
func TestGUISettingsRoundTripPreservesFalseSecurityOptions(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := loadConfig()
	cfg.FakeDNS = false
	cfg.PaddingEnabled = false
	cfg.ChaffingEnabled = false
	cfg.BlockIPv6 = false
	cfg.MinPadding = 0
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	got := loadConfig()
	if got.FakeDNS || got.PaddingEnabled || got.ChaffingEnabled || got.BlockIPv6 || got.MinPadding != 0 {
		t.Fatal("explicit false setting lost")
	}
}
func TestLegacyUserProfileIsPreservedNotReplacedWithExample(t *testing.T) {
	t.Chdir(t.TempDir())
	const data = `{"server":"198.51.100.20:443","pbk":"unit-test-value","user":"custom-user","sni":"example.com"}`
	if err := os.WriteFile("atlant_config.json", []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := loadConfig()
	if cfg.Server != "198.51.100.20:443" || cfg.PBK != "unit-test-value" || len(cfg.Servers) != 1 || cfg.Servers[0].Server != cfg.Server {
		t.Fatal("legacy user profile changed")
	}
}
