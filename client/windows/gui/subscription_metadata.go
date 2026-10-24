package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func effectiveWireVersion(v int) int {
	if v == 0 {
		return 4
	}
	return v
}
func parseWireVersion(value string) int {
	if value == "" {
		return 4
	}
	v, err := strconv.Atoi(value)
	if err != nil || (v != 2 && v != 3 && v != 4 && v != 5 && v != 6 && v != 7) {
		return -1
	}
	return v
}

func subscriptionHeaderText(value string) string {
	if len(value) > 4096 {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(value), "base64:") {
		decoded, err := base64.StdEncoding.DecodeString(value[7:])
		if err != nil {
			return ""
		}
		value = string(decoded)
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}

func applySubscriptionHeaders(s *Subscription, h http.Header) {
	if title := subscriptionHeaderText(h.Get("Profile-Title")); title != "" {
		s.Name = title
	}
	s.Announcement = subscriptionHeaderText(h.Get("Announce"))
	s.SupportURL = h.Get("Support-Url")
	s.UsageScope = h.Get("X-Subscription-Usage-Scope")
	s.UpdateHours, _ = strconv.Atoi(h.Get("Profile-Update-Interval"))
	if s.UpdateHours < 1 || s.UpdateHours > 168 {
		s.UpdateHours = 1
	}
}

func profileIdentity(p ServerProfile) string {
	if p.ServerID != "" && p.InboundID != "" {
		return "native\x00" + p.ServerID + "\x00" + p.InboundID
	}
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%s", p.Server, p.SNI, p.User, p.Transport, effectiveWireVersion(p.WireVersion), p.Path)
}

// The first native feed replaces a copied endpoint with a stable reference.
// Match that one-way upgrade without accepting a different pinned server.
func migratedSubscriptionProfile(previous, next ServerProfile) bool {
	return (previous.ServerID == "" || previous.InboundID == "") && next.ServerID != "" && next.InboundID != "" &&
		strings.EqualFold(previous.Server, next.Server) && strings.EqualFold(previous.SNI, next.SNI) && previous.User == next.User &&
		effectiveWireVersion(previous.WireVersion) == effectiveWireVersion(next.WireVersion) &&
		(previous.ServerPubKey == "" || strings.EqualFold(previous.ServerPubKey, next.ServerPubKey))
}

func subscriptionTransports(raw string) ([]string, bool) {
	if raw == "" {
		return nil, true
	}
	var result []string
	seen := map[string]bool{}
	for _, value := range strings.Split(raw, ",") {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "tcp" && value != "xhttp" && value != "quic" {
			return nil, false
		}
		if !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	return result, true
}

func validSubscriptionPath(path string) bool {
	return path == "" || len(path) <= 2048 && strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") &&
		!strings.ContainsAny(path, "?#\\") && !strings.ContainsFunc(path, unicode.IsControl)
}

// Selecting an explicit node must not keep another node's transport. Global
// routing and split-mode preferences remain independent of this selection.
func applyProfileTransport(cfg *AppConfig, p ServerProfile) {
	// Native profiles advertise capabilities; the client owns the transport choice.
	if len(p.Transports) > 0 {
		return
	}
	if p.Transport == "" {
		return
	}
	cfg.Transport = p.Transport
	switch p.Transport {
	case "tcp", "reality":
		cfg.Transport = "reality"
		cfg.TCPProto = "reality"
	case "xhttp":
		cfg.TCPProto = "xhttp"
	}
}

func remainingSubscriptionBytes(up, down, total int64) int64 {
	if total <= 0 || up >= total || down >= total-up {
		return 0
	}
	return total - up - down
}

func validateSubscriptionURL(value string) error {
	u, err := url.Parse(value)
	if err != nil {
		return errors.New("invalid subscription URL")
	}
	h := strings.ToLower(u.Hostname())
	loopback := h == "localhost" || h == "127.0.0.1" || h == "::1"
	if u.User != nil || h == "" || (u.Scheme != "https" && !(u.Scheme == "http" && loopback)) {
		return errors.New("для подписки требуется HTTPS с проверяемым сертификатом")
	}
	return nil
}

func subscriptionHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("too many subscription redirects")
		}
		if err := validateSubscriptionURL(r.URL.String()); err != nil {
			return err
		}
		if len(via) > 0 && via[0].URL.Scheme == "https" && r.URL.Scheme != "https" {
			return errors.New("HTTPS downgrade refused")
		}
		return nil
	}}
}
