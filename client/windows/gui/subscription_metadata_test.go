package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestSubscriptionTitleAndMultipleTransportsPreserved(t *testing.T) {
	h := http.Header{"Profile-Title": []string{"base64:0JzQvtC5IFZQTg=="}, "Profile-Update-Interval": []string{"2"}, "Announce": []string{"test announcement"}}
	var s Subscription
	applySubscriptionHeaders(&s, h)
	if s.Name != "Мой VPN" || s.UpdateHours != 2 || s.Announcement == "" {
		t.Fatal("header metadata lost", s.Name)
	}
	body := "atlant://198.51.100.20:443?user=alice&transport=reality&wire_version=3\natlant://198.51.100.20:443?user=alice&transport=quic&wire_version=3"
	profiles := parseSubscriptionLinks(body)
	if len(profiles) != 2 {
		t.Fatal("same-server transports merged")
	}
	profiles[0].PBK = "updated"
	cfg := &AppConfig{Servers: []ServerProfile{{ID: "manual", Name: "standalone"}, {ID: "sub-old", IsSubscription: true, Server: profiles[0].Server, SNI: profiles[0].SNI, User: "alice", Transport: "reality", WireVersion: 3, PBK: "old"}}, ActiveServerID: "sub-old"}
	cfg.Subscription = Subscription{URL: "https://example.com/sub/a", Name: "A"}
	normalizeSubscriptions(cfg)
	upsertSubscription(cfg, cfg.Subscriptions[0], profiles)
	if len(cfg.Servers) != 3 || cfg.PBK != "updated" || cfg.WireVersion != 3 {
		t.Fatal("refresh retained stale credentials or removed manual profile")
	}
	if len(parseSubscriptionLinks("atlant://198.51.100.20:443?wire_version=999")) != 0 {
		t.Fatal("unknown format downgraded")
	}
	if len(parseSubscriptionLinks(strings.Repeat(body+"\n", 200))) != 0 {
		t.Fatal("oversized profile count accepted")
	}
}

func TestSubscriptionURLPolicyAndCounterOverflow(t *testing.T) {
	for _, s := range []string{"https://example.com/sub/x", "https://198.51.100.20:8443/sub/x", "http://127.0.0.1/sub/test"} {
		if err := validateSubscriptionURL(s); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []string{"http://198.51.100.20/sub/x", "https://user:password@example.com/", "ftp://example.com/x"} {
		if validateSubscriptionURL(s) == nil {
			t.Fatal("unsafe subscription URL accepted")
		}
	}
	if remainingSubscriptionBytes(1<<62, 1<<62, 100) != 0 {
		t.Fatal("counter overflow")
	}
	if remainingSubscriptionBytes(5, 10, 100) != 85 {
		t.Fatal("incorrect remaining quota")
	}
}

func TestExplicitProfileTransportReplacesPreviousNode(t *testing.T) {
	cfg := &AppConfig{Transport: "xhttp", TCPProto: "xhttp", TrafficMode: "split", UDPProto: "quic"}
	applyProfileTransport(cfg, ServerProfile{Transport: "reality"})
	if cfg.Transport != "reality" || cfg.TCPProto != "reality" || cfg.TrafficMode != "split" || cfg.UDPProto != "quic" {
		t.Fatal("previous transport retained or unrelated routing changed")
	}
	applyProfileTransport(cfg, ServerProfile{Transport: "quic"})
	if cfg.Transport != "quic" {
		t.Fatal("QUIC node selection ignored")
	}
	applyProfileTransport(cfg, ServerProfile{Transport: "xhttp"})
	if cfg.Transport != "xhttp" || cfg.TCPProto != "xhttp" {
		t.Fatal("XHTTP node selection ignored")
	}
}

func TestNativeSubscriptionRefreshKeepsSelectionAndClientTransports(t *testing.T) {
	old := ServerProfile{ID: "selected", Server: "192.0.2.1:443", SNI: "old.example", User: "alice", PBK: "old", ServerID: "vds-one", InboundID: "in-one", SubscriptionID: "group", Transports: []string{"tcp", "xhttp", "quic"}}
	cfg := &AppConfig{Servers: []ServerProfile{old}, ActiveServerID: old.ID, Transport: "xhttp", TCPProto: "xhttp", UDPProto: "quic", TrafficMode: "split"}
	incoming := old
	incoming.Server, incoming.SNI, incoming.PBK, incoming.Path = "192.0.2.2:8443", "new.example", "rotated", "/custom/upload"
	replaceSubscriptionGroup(cfg, Subscription{ID: "group", Name: "My subscription"}, []ServerProfile{incoming})
	if cfg.ActiveServerID == "" || cfg.Server != incoming.Server || cfg.PBK != incoming.PBK || cfg.Path != incoming.Path {
		t.Fatal("native identity refresh lost selection or endpoint settings")
	}
	if cfg.Transport != "xhttp" || cfg.TCPProto != "xhttp" || cfg.UDPProto != "quic" || cfg.TrafficMode != "split" {
		t.Fatal("native feed overrode client transport choice")
	}
	stableID := cfg.ActiveServerID
	incoming.Server, incoming.PBK = "192.0.2.3:443", "rotated-again"
	replaceSubscriptionGroup(cfg, Subscription{ID: "group"}, []ServerProfile{incoming})
	if cfg.ActiveServerID != stableID {
		t.Fatal("native profile ID changed with credentials/address")
	}
	selectSubscriptionProfile(cfg, ServerProfile{ID: "manual"})
	if cfg.Path != "" {
		t.Fatal("previous inbound path leaked to a different profile")
	}
}

func TestNativeSubscriptionMetadataParsing(t *testing.T) {
	base := "atlant://192.0.2.1:443?wire_version=7&server_id=vds&inbound_id=in&path=%2Fcustom%2Fupload&transports=tcp,xhttp,quic"
	profiles := parseSubscriptionLinks(base)
	if len(profiles) != 1 || profiles[0].ServerID != "vds" || profiles[0].InboundID != "in" || profiles[0].Path != "/custom/upload" || len(profiles[0].Transports) != 3 {
		t.Fatal("native subscription metadata missing")
	}
	for _, suffix := range []string{"&path=%2F%2Fexternal.example", "&path=%2Fbad%3Fquery", "&transports=tcp,unknown"} {
		if len(parseSubscriptionLinks(strings.Split(base, "&path=")[0]+suffix)) != 0 {
			t.Fatal("invalid native metadata accepted")
		}
	}
}

func TestMigrationFeedRetainsSelectedEndpointAndClientTransport(t *testing.T) {
	old := ServerProfile{ID: "selected", SubscriptionID: "group", Server: "192.0.2.1:443", SNI: "example.com", User: "alice", ServerPubKey: "pin", WireVersion: 7, Transport: "quic"}
	cfg := &AppConfig{Servers: []ServerProfile{old}, ActiveServerID: old.ID, TCPProto: "xhttp", Transport: "xhttp", UDPProto: "quic", TrafficMode: "split"}
	next := old
	next.ServerID = "vds"
	next.InboundID = "local"
	next.Transport = "reality"
	next.Transports = []string{"tcp", "xhttp", "quic"}
	next.PBK = "rotated"
	remote := next
	remote.InboundID = "remote"
	remote.Server = "192.0.2.2:443"
	replaceSubscriptionGroup(cfg, Subscription{ID: "group"}, []ServerProfile{remote, next})
	if cfg.ActiveServerID == "" || cfg.Server != old.Server || cfg.PBK != "rotated" || cfg.TCPProto != "xhttp" || cfg.Transport != "xhttp" {
		t.Fatal("migration cleared selection or switched to the first remote profile")
	}
	next.ServerPubKey = "another pin"
	if migratedSubscriptionProfile(old, next) {
		t.Fatal("migration matched a different pinned server")
	}
}
