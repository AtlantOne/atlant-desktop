package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestSubscriptionRefreshKeepsNameAndCatalogPosition(t *testing.T) {
	a := Subscription{ID: "a", URL: "https://example.com/a", Name: "Моё название"}
	b := Subscription{ID: "b", URL: "https://example.com/b", Name: "Вторая"}
	cfg := &AppConfig{}
	upsertSubscription(cfg, a, []ServerProfile{{Server: "one:443", User: "alice"}})
	upsertSubscription(cfg, b, []ServerProfile{{Server: "two:443", User: "bob"}})
	cfg.Servers = append(cfg.Servers, ServerProfile{ID: "manual", Server: "manual:443"})
	selected := cfg.Servers[0].ID
	cfg.ActiveServerID = selected
	for i := 0; i < 3; i++ {
		upsertSubscription(cfg, Subscription{URL: a.URL, Name: "Заголовок провайдера", Upload: int64(i + 1)}, []ServerProfile{{Server: "one:443", User: "alice"}, {Server: "one:8443", User: "alice"}})
		if cfg.Subscriptions[0].ID != "a" || cfg.Subscriptions[1].ID != "b" || cfg.Subscriptions[0].Name != a.Name || cfg.Subscriptions[0].Upload != int64(i+1) {
			t.Fatal("refresh moved the subscription, replaced its name, or lost metadata")
		}
		if cfg.Servers[0].SubscriptionID != "a" || cfg.Servers[1].SubscriptionID != "a" || cfg.Servers[2].SubscriptionID != "b" || cfg.Servers[3].ID != "manual" || cfg.ActiveServerID != selected {
			t.Fatal("refresh moved the server group or lost selection")
		}
		if cfg.Servers[0].Group != a.Name {
			t.Fatal("server group did not keep local name")
		}
	}
}

func TestSubscriptionGroupsIndependent(t *testing.T) {
	cfg := &AppConfig{Subscription: Subscription{URL: "https://example.com/a", Name: "Same"}, Servers: []ServerProfile{{ID: "old", IsSubscription: true, Server: "one:443", User: "alice"}, {ID: "manual", Server: "manual:443"}}}
	normalizeSubscriptions(cfg)
	a := cfg.Subscriptions[0]
	cfg.ActiveServerID = "old"
	b := Subscription{ID: subscriptionID("https://example.com/b"), URL: "https://example.com/b", Name: "Same"}
	p := ServerProfile{Server: "one:443", User: "alice"}
	upsertSubscription(cfg, b, []ServerProfile{p})
	if len(cfg.Subscriptions) != 2 || len(cfg.Servers) != 3 || cfg.ActiveServerID != "old" {
		t.Fatalf("adding B replaced A: %+v", cfg)
	}
	upsertSubscription(cfg, a, []ServerProfile{p})
	if len(cfg.Servers) != 3 {
		t.Fatal("updating A removed B")
	}
	var aid, bid string
	for _, p := range cfg.Servers {
		if p.SubscriptionID == a.ID {
			aid = p.ID
		}
		if p.SubscriptionID == b.ID {
			bid = p.ID
		}
	}
	if aid == "" || bid == "" || aid == bid || cfg.ActiveServerID != aid {
		t.Fatal("IDs/selection not scoped")
	}
	replaceSubscriptionGroup(cfg, b, nil)
	if len(cfg.Servers) != 2 || cfg.ActiveServerID != aid {
		t.Fatal("removing B changed A")
	}
}

func TestSubscriptionHTTPSelectedGroupOnly(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("atlant_config.json", []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &AppConfig{}
	upsertSubscription(cfg, Subscription{ID: "a", URL: "https://example.com/a", Name: "Same"}, []ServerProfile{{Server: "one:443"}})
	upsertSubscription(cfg, Subscription{ID: "b", URL: "https://example.com/b", Name: "Same"}, []ServerProfile{{Server: "two:443"}})
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	call := func(h http.HandlerFunc, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		return w
	}
	if w := call(clearSubscriptionHTTP, `{}`); w.Code != 400 {
		t.Fatal("ambiguous delete accepted", w.Code)
	}
	if len(loadConfig().Subscriptions) != 2 {
		t.Fatal("ambiguous delete changed catalog")
	}
	if w := call(editSubscriptionHTTP, `{"id":"b","name":"Renamed","url":"https://example.com/b2"}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	updated := loadConfig()
	if updated.Subscriptions[0].Name != "Same" || updated.Subscriptions[1].Name != "Renamed" || updated.Subscriptions[1].ID != "b" {
		t.Fatal("rename affected another group or changed stable ID")
	}
	if w := call(editSubscriptionHTTP, `{"id":"b","url":"https://example.com/a"}`); w.Code != 409 {
		t.Fatal("duplicate URL accepted")
	}
	if w := call(editSubscriptionHTTP, `{"id":"b","url":"http://example.com/plain"}`); w.Code != 400 {
		t.Fatal("insecure URL accepted")
	}
	if w := call(clearSubscriptionHTTP, `{"id":"a"}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	left := loadConfig()
	if len(left.Subscriptions) != 1 || left.Subscriptions[0].ID != "b" || len(left.Servers) != 1 || left.Servers[0].Server != "two:443" {
		t.Fatal("clear crossed group boundary")
	}
	if w := call(clearSubscriptionHTTP, `{"id":"b"}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if len(loadConfig().Subscriptions) != 0 {
		t.Fatal("legacy mirror resurrected deleted subscription")
	}
}

func TestSubscriptionBatchFetchAndAtomicFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			http.Error(w, "fail", 500)
			return
		}
		w.Header().Set("Profile-Title", "Same title")
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=107374182; expire=0")
		w.Write([]byte("atlant://alice@127.0.0.1:443?sni=example.com&pbk=" + strings.Repeat("a", 64) + "&server_pub_key=" + strings.Repeat("b", 64) + "&wire_version=7#Node"))
	}))
	defer srv.Close()
	cfg := &AppConfig{}
	if _, err := importSubscriptionURLs(cfg, srv.URL+"/a\n"+srv.URL+"/bad"); err == nil {
		t.Fatal("accepted failed batch")
	}
	if len(cfg.Subscriptions) != 0 || len(cfg.Servers) != 0 {
		t.Fatal("partial mutation")
	}
	if n, err := importSubscriptionURLs(cfg, srv.URL+"/a\n"+srv.URL+"/b\n"+srv.URL+"/a"); err != nil || n != 2 {
		t.Fatalf("batch: %d %v", n, err)
	}
	if len(cfg.Subscriptions) != 2 || len(cfg.Servers) != 2 {
		t.Fatal("same endpoint in different groups collapsed")
	}
	raw, _ := json.Marshal(cfg)
	var restored AppConfig
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	normalizeSubscriptions(&restored)
	if len(restored.Subscriptions) != 2 || restored.Subscriptions[1].Total != 107374182 {
		t.Fatal("lost quota/groups")
	}
}

func TestWireSevenImport(t *testing.T) {
	if parseWireVersion("7") != 7 || parseWireVersion("6") != 6 || parseWireVersion("77") != -1 {
		t.Fatal("wire parser")
	}
}

func TestStaleSettingsCannotOverwriteSubscriptions(t *testing.T) {
	current := &AppConfig{Servers: []ServerProfile{{ID: "new"}}, Subscriptions: []Subscription{{ID: "s"}}, ActiveServerID: "new", Server: "new:443"}
	next := &AppConfig{Servers: []ServerProfile{{ID: "old"}}, MTU: 1400, TCPProto: "xhttp"}
	preserveGUICatalog(next, current)
	if next.Servers[0].ID != "new" || next.Subscriptions[0].ID != "s" || next.Server != "new:443" || next.MTU != 1400 || next.TCPProto != "xhttp" {
		t.Fatal("stale form overwrote catalog or settings lost")
	}
}
