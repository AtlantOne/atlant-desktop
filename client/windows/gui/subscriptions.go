package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func subscriptionID(raw string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return fmt.Sprintf("subscription-%x", h[:12])
}

// Settings submissions may have been prepared before a subscription refresh.
// Profile mutations have dedicated endpoints; a stale settings form cannot undo them.
func preserveGUICatalog(next, current *AppConfig) {
	next.Servers, next.Subscriptions, next.Subscription = current.Servers, current.Subscriptions, current.Subscription
	next.ActiveServerID = current.ActiveServerID
	next.Server, next.SNI, next.User = current.Server, current.SNI, current.User
	next.PBK, next.ServerPubKey, next.WireVersion = current.PBK, current.ServerPubKey, current.WireVersion
	next.Path = current.Path
}

// Keep the old field as a read-compatible mirror, never as the owner of every node.
func normalizeSubscriptions(cfg *AppConfig) {
	if len(cfg.Subscriptions) == 0 && cfg.Subscription.URL != "" {
		cfg.Subscriptions = []Subscription{cfg.Subscription}
	}
	for i := range cfg.Subscriptions {
		if cfg.Subscriptions[i].ID == "" {
			cfg.Subscriptions[i].ID = subscriptionID(cfg.Subscriptions[i].URL)
		}
	}
	if len(cfg.Subscriptions) == 1 {
		for i := range cfg.Servers {
			p := &cfg.Servers[i]
			legacyMember := p.IsSubscription || p.Group == cfg.Subscriptions[0].Name && strings.HasPrefix(p.ID, "sub-")
			if legacyMember && p.SubscriptionID == "" {
				p.IsSubscription = true
				p.SubscriptionID = cfg.Subscriptions[0].ID
			}
		}
	}
	if len(cfg.Subscriptions) > 0 {
		cfg.Subscription = cfg.Subscriptions[0]
	} else {
		cfg.Subscription = Subscription{}
	}
}

func findSubscription(cfg *AppConfig, id, rawURL string) int {
	for i, sub := range cfg.Subscriptions {
		if id != "" && sub.ID == id || id == "" && rawURL != "" && sub.URL == rawURL {
			return i
		}
	}
	if id == "" && rawURL == "" && len(cfg.Subscriptions) == 1 {
		return 0
	}
	return -1
}

func selectSubscriptionProfile(cfg *AppConfig, p ServerProfile) {
	cfg.ActiveServerID, cfg.Server, cfg.SNI = p.ID, p.Server, p.SNI
	cfg.User, cfg.PBK, cfg.ServerPubKey, cfg.WireVersion = p.User, p.PBK, p.ServerPubKey, p.WireVersion
	cfg.Path = p.Path
	applyProfileTransport(cfg, p)
}

func selectFirstProfileIfUnset(cfg *AppConfig) {
	if cfg.ActiveServerID != "" || cfg.Server != "" {
		return
	}
	for _, p := range cfg.Servers {
		if p.ID != "" && p.Server != "" {
			selectSubscriptionProfile(cfg, p)
			return
		}
	}
}

func clearSelectedProfile(cfg *AppConfig) {
	cfg.ActiveServerID, cfg.Server, cfg.SNI, cfg.PBK, cfg.User, cfg.ServerPubKey = "", "", "", "", "", ""
	cfg.Path = ""
}

func replaceSubscriptionGroup(cfg *AppConfig, sub Subscription, profiles []ServerProfile) {
	selected, owned := "", false
	var previousProfile ServerProfile
	kept := make([]ServerProfile, 0, len(cfg.Servers)+len(profiles))
	position := -1
	for _, p := range cfg.Servers {
		if p.SubscriptionID == sub.ID {
			if position < 0 {
				position = len(kept)
			}
			if p.ID == cfg.ActiveServerID {
				selected, owned = profileIdentity(p), true
				previousProfile = p
			}
		} else {
			kept = append(kept, p)
		}
	}
	matched := false
	seen := map[string]bool{}
	replacement := make([]ServerProfile, 0, len(profiles))
	for _, p := range profiles {
		identity := profileIdentity(p)
		if seen[identity] {
			continue
		}
		seen[identity] = true
		h := sha256.Sum256([]byte(sub.ID + "\x00" + identity))
		p.ID, p.SubscriptionID, p.IsSubscription, p.Group = fmt.Sprintf("sub-%x", h[:12]), sub.ID, true, sub.Name
		replacement = append(replacement, p)
		if owned && (selected == identity || migratedSubscriptionProfile(previousProfile, p)) {
			selectSubscriptionProfile(cfg, p)
			matched = true
		}
	}
	if owned && !matched {
		clearSelectedProfile(cfg)
	}
	if position < 0 {
		position = len(kept)
	}
	result := make([]ServerProfile, 0, len(kept)+len(replacement))
	result = append(result, kept[:position]...)
	result = append(result, replacement...)
	cfg.Servers = append(result, kept[position:]...)
}

func fetchSubscription(raw string) (Subscription, []ServerProfile, error) {
	if err := validateSubscriptionURL(raw); err != nil {
		return Subscription{}, nil, err
	}
	resp, err := subscriptionHTTPClient().Get(raw)
	if err != nil {
		return Subscription{}, nil, errors.New("не удалось загрузить подписку")
	}
	defer resp.Body.Close()
	body, err := readSubscriptionResponse(resp)
	if err != nil {
		return Subscription{}, nil, err
	}
	profiles := parseSubscriptionLinks(string(body))
	if len(profiles) == 0 {
		return Subscription{}, nil, errors.New("подписка не содержит профилей Atlant; прежние серверы сохранены")
	}
	u, _ := url.Parse(raw)
	s := Subscription{ID: subscriptionID(raw), URL: raw, Name: u.Host, LastUpdated: time.Now().Format("02.01.2006 15:04"), Count: len(profiles)}
	s.Upload, s.Download, s.Total, s.Expire = parseSubscriptionUserinfo(resp.Header)
	applySubscriptionHeaders(&s, resp.Header)
	if s.Total > 0 {
		s.RemainingGB = fmt.Sprintf("Осталось %.2f / %.2f ГиБ", float64(remainingSubscriptionBytes(s.Upload, s.Download, s.Total))/1073741824, float64(s.Total)/1073741824)
	} else {
		s.RemainingGB = "Безлимит"
	}
	return s, profiles, nil
}

func upsertSubscription(cfg *AppConfig, sub Subscription, profiles []ServerProfile) {
	i := findSubscription(cfg, "", sub.URL)
	if i >= 0 {
		sub.ID = cfg.Subscriptions[i].ID
		if cfg.Subscriptions[i].Name != "" {
			sub.Name = cfg.Subscriptions[i].Name
		}
		cfg.Subscriptions[i] = sub
	} else {
		cfg.Subscriptions = append(cfg.Subscriptions, sub)
	}
	replaceSubscriptionGroup(cfg, sub, profiles)
	normalizeSubscriptions(cfg)
}

func subscriptionURLs(text string) ([]string, error) {
	parts := strings.Fields(text)
	if len(parts) == 0 || len(parts) > 32 {
		return nil, errors.New("за один импорт можно добавить от 1 до 32 подписок")
	}
	urls := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		if err := validateSubscriptionURL(p); err != nil {
			return nil, err
		}
		if !seen[p] {
			seen[p] = true
			urls = append(urls, p)
		}
	}
	return urls, nil
}

func importSubscriptionURLs(cfg *AppConfig, text string) (int, error) {
	urls, err := subscriptionURLs(text)
	if err != nil {
		return 0, err
	}
	type fetched struct {
		sub      Subscription
		profiles []ServerProfile
	}
	all := []fetched{}
	for _, raw := range urls {
		s, p, err := fetchSubscription(raw)
		if err != nil {
			return 0, err
		}
		all = append(all, fetched{s, p})
	}
	count := 0
	// Apply only after every URL was validated and fetched; a failed batch is atomic.
	for _, f := range all {
		upsertSubscription(cfg, f.sub, f.profiles)
		count += len(f.profiles)
	}
	return count, nil
}

func writeSubscriptionResponse(w http.ResponseWriter, cfg *AppConfig, count int) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "count": count, "added_count": count, "servers": cfg.Servers, "subscription": cfg.Subscription, "subscriptions": cfg.Subscriptions})
}

func updateSubscriptionHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct{ ID, URL string }
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "Некорректный запрос", 400)
		return
	}
	cfg := loadConfig()
	if req.ID != "" || strings.TrimSpace(req.URL) == "" {
		i := findSubscription(cfg, req.ID, "")
		if i < 0 {
			http.Error(w, "Выберите подписку", 400)
			return
		}
		req.URL = cfg.Subscriptions[i].URL
	}
	count, err := importSubscriptionURLs(cfg, req.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if commitGUIConfig(w, cfg) {
		writeSubscriptionResponse(w, cfg, count)
	}
}

func clearSubscriptionHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct{ ID string }
	if r.ContentLength != 0 && json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "Некорректный запрос", 400)
		return
	}
	cfg := loadConfig()
	i := findSubscription(cfg, req.ID, "")
	if i < 0 {
		http.Error(w, "Выберите подписку", 400)
		return
	}
	sub := cfg.Subscriptions[i]
	replaceSubscriptionGroup(cfg, sub, nil)
	cfg.Subscriptions = append(cfg.Subscriptions[:i], cfg.Subscriptions[i+1:]...)
	cfg.Subscription = Subscription{}
	if commitGUIConfig(w, cfg) {
		writeSubscriptionResponse(w, cfg, 0)
	}
}

func editSubscriptionHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct{ ID, Name, URL string }
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "Некорректный запрос", 400)
		return
	}
	cfg := loadConfig()
	i := findSubscription(cfg, req.ID, "")
	if i < 0 {
		http.Error(w, "Выберите подписку", 400)
		return
	}
	if raw := strings.TrimSpace(req.URL); raw != "" {
		if err := validateSubscriptionURL(raw); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if other := findSubscription(cfg, "", raw); other >= 0 && other != i {
			http.Error(w, "Эта подписка уже добавлена", 409)
			return
		}
		cfg.Subscriptions[i].URL = raw
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		if len(name) > 256 {
			http.Error(w, "Слишком длинное название", 400)
			return
		}
		cfg.Subscriptions[i].Name = name
		for j := range cfg.Servers {
			if cfg.Servers[j].SubscriptionID == cfg.Subscriptions[i].ID {
				cfg.Servers[j].Group = name
			}
		}
	}
	if commitGUIConfig(w, cfg) {
		writeSubscriptionResponse(w, cfg, 0)
	}
}
