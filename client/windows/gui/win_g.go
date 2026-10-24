package main

import (
	"bufio"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

//go:embed web/*
var webFS embed.FS

var (
	coreCmd       *exec.Cmd
	coreDone      chan struct{}
	coreStdin     io.WriteCloser
	cmdMu         sync.Mutex
	isConnected   bool
	logMu         sync.Mutex
	logBuffer     []string
	maxLogs       = 500
	savedGateway  string
	activeConfig  *AppConfig
	lastHeartbeat = time.Now()
	heartbeatMu   sync.Mutex
	browserCmd    *exec.Cmd
)

type ServerProfile struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Server         string                 `json:"server"`
	SNI            string                 `json:"sni"`
	PBK            string                 `json:"pbk"`
	ServerPubKey   string                 `json:"server_pub_key"`
	WireVersion    int                    `json:"wire_version,omitempty"`
	IsSubscription bool                   `json:"is_subscription,omitempty"`
	SubscriptionID string                 `json:"subscription_id,omitempty"`
	User           string                 `json:"user"`
	Transport      string                 `json:"transport"`
	TrafficMode    string                 `json:"traffic_mode"`
	Security       string                 `json:"security,omitempty"`
	Group          string                 `json:"group,omitempty"`
	Type           string                 `json:"type,omitempty"`
	RawJSON        string                 `json:"raw_json,omitempty"`
	CustomConfig   map[string]interface{} `json:"custom_config,omitempty"`
	Ping           int64                  `json:"ping,omitempty"`
	Fingerprint    string                 `json:"fingerprint,omitempty"`
	ServerID       string                 `json:"server_id,omitempty"`
	InboundID      string                 `json:"inbound_id,omitempty"`
	Path           string                 `json:"path,omitempty"`
	Transports     []string               `json:"transports,omitempty"`
}

type Subscription struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	Name         string `json:"name,omitempty"`
	LastUpdated  string `json:"last_updated"`
	Count        int    `json:"count"`
	Upload       int64  `json:"upload,omitempty"`
	Download     int64  `json:"download,omitempty"`
	Total        int64  `json:"total,omitempty"`
	Expire       int64  `json:"expire,omitempty"`
	RemainingGB  string `json:"remaining_gb,omitempty"`
	UpdateHours  int    `json:"update_hours,omitempty"`
	SupportURL   string `json:"support_url,omitempty"`
	Announcement string `json:"announcement,omitempty"`
	UsageScope   string `json:"usage_scope,omitempty"`
}

type AppConfig struct {
	ActiveServerID   string          `json:"active_server_id"`
	Server           string          `json:"server"`
	SNI              string          `json:"sni"`
	PBK              string          `json:"pbk"`
	ServerPubKey     string          `json:"server_pub_key"`
	Path             string          `json:"path,omitempty"`
	WireVersion      int             `json:"wire_version,omitempty"`
	User             string          `json:"user"`
	LocalPort        string          `json:"local,omitempty"`
	Transport        string          `json:"transport,omitempty"`
	TrafficMode      string          `json:"traffic_mode,omitempty"`
	UDPProto         string          `json:"udp_proto,omitempty"`
	TCPProto         string          `json:"tcp_proto,omitempty"`
	Security         string          `json:"security,omitempty"`
	RouteMode        string          `json:"route_mode,omitempty"`
	ProxyApps        []string        `json:"proxy_apps,omitempty"`
	BypassApps       []string        `json:"bypass_apps,omitempty"`
	ProxyDomains     []string        `json:"proxy_domains,omitempty"`
	DirectDomains    []string        `json:"direct_domains,omitempty"`
	Tun              bool            `json:"tun"`
	KillSwitch       bool            `json:"kill_switch"`
	MTU              int             `json:"mtu,omitempty"`
	DNS              string          `json:"dns,omitempty"`
	AllowLAN         bool            `json:"allow_lan,omitempty"`
	BlockIPv6        bool            `json:"block_ipv6"`
	Mux              bool            `json:"mux,omitempty"`
	FakeDNS          bool            `json:"fake_dns"`
	TrafficPadding   bool            `json:"traffic_padding"`
	LogLevel         string          `json:"log_level,omitempty"`
	UpMbps           int             `json:"up_mbps,omitempty"`
	DownMbps         int             `json:"down_mbps,omitempty"`
	Servers          []ServerProfile `json:"servers,omitempty"`
	Subscription     Subscription    `json:"subscription,omitempty"`
	Subscriptions    []Subscription  `json:"subscriptions,omitempty"`
	Fingerprint      string          `json:"fingerprint,omitempty"`
	PaddingEnabled   bool            `json:"padding_enabled"`
	MinPadding       int             `json:"min_padding"`
	MaxPadding       int             `json:"max_padding"`
	ChaffingEnabled  bool            `json:"chaffing_enabled"`
	ChaffingInterval int             `json:"chaffing_interval,omitempty"`
	ChaffingMinBytes int             `json:"chaffing_min_bytes,omitempty"`
	ChaffingMaxBytes int             `json:"chaffing_max_bytes,omitempty"`
}

func getConfigFile() string {
	if _, err := os.Stat("atlant_config.json"); err == nil {
		return "atlant_config.json"
	}
	ex, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(ex)
		p := filepath.Join(exeDir, "atlant_config.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Join(filepath.Dir(exeDir), "atlant_config.json")
		if _, err := os.Stat(parent); err == nil {
			return parent
		}
	}
	return "atlant_config.json"
}

func loadConfig() *AppConfig {
	cfg := &AppConfig{
		WireVersion: 4,
		LocalPort:   "127.0.0.1:1080", Transport: "reality", TrafficMode: "split",
		RouteMode: "all", Tun: true, MTU: 1360, DNS: "1.1.1.1", BlockIPv6: true,
		Fingerprint: "firefox", PaddingEnabled: true, MinPadding: 32, MaxPadding: 256,
		ChaffingEnabled: true, ChaffingInterval: 10, ChaffingMinBytes: 16, ChaffingMaxBytes: 64,
		FakeDNS: true, TrafficPadding: true,
	}

	data, err := os.ReadFile(getConfigFile())
	if err == nil {
		plain, legacy, decodeErr := decodeStoredConfig(data)
		if decodeErr != nil {
			appendLog("[ERR] Не удалось расшифровать настройки. Исходный файл сохранён; проверьте учётную запись Windows.")
			return cfg
		}
		if json.Unmarshal(plain, cfg) != nil {
			appendLog("[ERR] Некорректный файл настроек; автоматическая перезапись запрещена.")
			return cfg
		}
		if legacy {
			if err := saveProtectedConfig(getConfigFile(), plain); err != nil {
				appendLog("[ERR] Не удалось защитить старые настройки Windows; исходный файл сохранён.")
			}
		}
	}
	if len(cfg.Servers) == 0 && cfg.Server != "" {
		cfg.Servers = append(cfg.Servers, ServerProfile{
			ID:           "srv-1",
			Name:         "Импортированный сервер",
			Server:       cfg.Server,
			SNI:          cfg.SNI,
			PBK:          cfg.PBK,
			ServerPubKey: cfg.ServerPubKey,
			Path:         cfg.Path,
			User:         cfg.User,
			Transport:    cfg.Transport,
			TrafficMode:  cfg.TrafficMode,
			Group:        "Мои серверы",
			Type:         "atlant",
		})
	}

	if cfg.LocalPort == "" {
		cfg.LocalPort = "127.0.0.1:1080"
	}
	normalizeSubscriptions(cfg)
	return cfg
}

func saveConfig(cfg *AppConfig) error {
	normalizeSubscriptions(cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return saveProtectedConfig(getConfigFile(), data)
}

func appendLog(line string) {
	logMu.Lock()
	defer logMu.Unlock()
	logBuffer = append(logBuffer, line)
	if len(logBuffer) > maxLogs {
		logBuffer = logBuffer[len(logBuffer)-maxLogs:]
	}
}

func setSystemProxy(enable bool, proxyAddr string) error {
	if !enable && originalProxy == nil {
		return nil
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if enable && proxyAddr != "" {
		if originalProxy == nil {
			originalProxy, err = captureProxy(k)
			if err != nil {
				return err
			}
		}
		if err = k.SetStringValue("ProxyServer", "socks="+proxyAddr); err == nil {
			err = k.SetStringValue("ProxyOverride", "<local>;localhost;127.*;10.*;172.16.*;172.17.*;172.18.*;172.19.*;172.20.*;192.168.*")
		}
		if err == nil {
			err = k.SetDWordValue("ProxyEnable", 1)
		}
		if err != nil {
			_ = restoreProxy(k, originalProxy)
			return err
		}
	} else {
		if err = restoreProxy(k, originalProxy); err != nil {
			return err
		}
		originalProxy = nil
	}

	wininet := windows.NewLazySystemDLL("wininet.dll")
	internetSetOption := wininet.NewProc("InternetSetOptionW")
	internetSetOption.Call(0, 39, 0, 0) // INTERNET_OPTION_SETTINGS_CHANGED
	internetSetOption.Call(0, 37, 0, 0) // INTERNET_OPTION_REFRESH
	return nil
}

func startCore() error {
	cmdMu.Lock()
	defer cmdMu.Unlock()

	if isConnected {
		return nil
	}

	cfg := loadConfig()
	if cfg.Server == "" || cfg.PBK == "" || cfg.User == "" || cfg.SNI == "" {
		return fmt.Errorf("Добавьте и выберите сервер перед подключением")
	}
	activeConfig = cfg

	ex, _ := os.Executable()
	exPath := filepath.Dir(ex)
	var coreExe string
	for _, candidate := range []string{
		filepath.Join(exPath, "atlant-win-fixed.exe"), filepath.Join(exPath, "atlant_win_64.exe"),
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			coreExe = candidate
			break
		}
	}
	if coreExe == "" {
		return fmt.Errorf("Atlant VPN core executable not found")
	}

	// Only the packaged DLL beside the core is eligible; no CWD/parent search.
	coreDir := filepath.Dir(coreExe)
	wintunTarget := filepath.Join(coreDir, "wintun.dll")
	if cfg.Tun {
		if info, err := os.Lstat(wintunTarget); err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("Положите штатный wintun.dll рядом с ядром VPN; поиск в сторонних папках отключён")
		}
	}

	args := []string{
		"-no-config",
		"-server", cfg.Server,
		"-sni", cfg.SNI,
		"-pbk-stdin",
		"-server-pub-key", cfg.ServerPubKey,
		"-wire-version", fmt.Sprint(effectiveWireVersion(cfg.WireVersion)),
		fmt.Sprintf("-fake-dns=%t", cfg.FakeDNS),
	}
	if cfg.User != "" {
		args = append(args, "-user", cfg.User)
	}
	if cfg.Path != "" {
		args = append(args, "-path", cfg.Path)
	}
	if cfg.LocalPort != "" {
		args = append(args, "-local", cfg.LocalPort)
	}
	if cfg.Transport != "" {
		args = append(args, "-transport", cfg.Transport)
	}
	if cfg.TrafficMode != "" {
		args = append(args, "-traffic-mode", cfg.TrafficMode)
	}
	if cfg.UDPProto != "" {
		args = append(args, "-udp-proto", cfg.UDPProto)
	}
	if cfg.TCPProto != "" {
		args = append(args, "-tcp-proto", cfg.TCPProto)
	}
	if cfg.RouteMode != "" {
		args = append(args, "-route-mode", cfg.RouteMode)
	}
	if len(cfg.ProxyApps) > 0 {
		args = append(args, "-proxy-apps", strings.Join(cfg.ProxyApps, ","))
	}
	if len(cfg.BypassApps) > 0 {
		args = append(args, "-bypass-apps", strings.Join(cfg.BypassApps, ","))
	}
	if len(cfg.ProxyDomains) > 0 {
		args = append(args, "-proxy-domains", strings.Join(cfg.ProxyDomains, ","))
	}
	if len(cfg.DirectDomains) > 0 {
		args = append(args, "-direct-domains", strings.Join(cfg.DirectDomains, ","))
	}
	mtuVal := cfg.MTU
	if mtuVal <= 0 {
		mtuVal = 1420
	}
	args = append(args, "-mtu", fmt.Sprint(mtuVal))

	dnsServer := cfg.DNS
	if dnsServer == "" {
		dnsServer = "1.1.1.1"
	}

	if cfg.Tun {
		args = append(args,
			"-tun-addr", "10.254.1.2",
			"-tun-gw", "10.254.1.1",
			"-tun-mask", "255.255.255.0",
			"-tun-dns", dnsServer,
			"-tun",
		)
	}
	if cfg.KillSwitch {
		args = append(args, "-kill-switch")
	}
	if cfg.AllowLAN {
		args = append(args, "-allow-lan")
	}
	if cfg.BlockIPv6 {
		args = append(args, "-block-ipv6")
	}
	if cfg.Mux {
		args = append(args, "-mux")
	}
	if cfg.LogLevel != "" {
		args = append(args, "-loglevel", cfg.LogLevel)
	}
	if cfg.UpMbps > 0 {
		args = append(args, "-up-mbps", fmt.Sprint(cfg.UpMbps))
	}
	if cfg.DownMbps > 0 {
		args = append(args, "-down-mbps", fmt.Sprint(cfg.DownMbps))
	}
	effectiveFp := cfg.Fingerprint
	for _, p := range cfg.Servers {
		if p.ID == cfg.ActiveServerID && p.Fingerprint != "" {
			effectiveFp = p.Fingerprint
			break
		}
	}
	if effectiveFp != "" {
		args = append(args, "-fingerprint", effectiveFp)
	}
	if cfg.PaddingEnabled {
		args = append(args, "-padding")
	} else {
		args = append(args, "-padding=false")
	}
	if cfg.MinPadding > 0 {
		args = append(args, "-min-padding", fmt.Sprint(cfg.MinPadding))
	}
	if cfg.MaxPadding > 0 {
		args = append(args, "-max-padding", fmt.Sprint(cfg.MaxPadding))
	}
	if cfg.ChaffingEnabled {
		args = append(args, "-chaffing")
	} else {
		args = append(args, "-chaffing=false")
	}
	if cfg.ChaffingInterval > 0 {
		args = append(args, "-chaffing-interval", fmt.Sprint(cfg.ChaffingInterval))
	}

	cmd := exec.Command(coreExe, args...)
	cmd.Dir = filepath.Dir(coreExe)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	coreStdin = stdin
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return err
	}
	if _, err := io.WriteString(stdin, cfg.PBK+"\n"); err != nil {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		coreStdin = nil
		return fmt.Errorf("Не удалось передать ключ ядру через приватный канал")
	}

	coreCmd = cmd
	done := make(chan struct{})
	coreDone = done
	isConnected = true
	if !cfg.Tun {
		proxyHost := cfg.LocalPort
		if strings.HasPrefix(proxyHost, ":") {
			proxyHost = "127.0.0.1" + proxyHost
		} else if !strings.Contains(proxyHost, ":") {
			proxyHost = "127.0.0.1:" + proxyHost
		}
		if err := setSystemProxy(true, proxyHost); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			close(done)
			coreCmd, coreDone, coreStdin = nil, nil, nil
			isConnected = false
			return fmt.Errorf("configure Windows proxy: %w", err)
		}
		appendLog("[GUI] Windows system proxy enabled -> " + proxyHost)
	} else {
		_ = setSystemProxy(false, "")
	}
	appendLog("[GUI] Atlant VPN core launched successfully")

	go streamOutput(stdout)
	go streamOutput(stderr)

	go func() {
		err := cmd.Wait()
		close(done)
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		cmdMu.Lock()
		if coreCmd == cmd {
			isConnected = false
			coreCmd = nil
			coreStdin = nil
			coreDone = nil
			_ = setSystemProxy(false, "")
		}
		cmdMu.Unlock()
		if err != nil {
			appendLog(fmt.Sprintf("[GUI] Atlant VPN core process exited: %v (code %d)", err, exitCode))
		} else {
			appendLog(fmt.Sprintf("[GUI] Atlant VPN core process exited (code %d)", exitCode))
		}
	}()

	return nil
}

func stopCore() {
	cmdMu.Lock()
	defer cmdMu.Unlock()

	_ = setSystemProxy(false, "")

	// Посылаем сигнал мягкой остановки в ядро
	if coreStdin != nil {
		_, _ = coreStdin.Write([]byte("STOP\n"))
		_ = coreStdin.Close()
		coreStdin = nil
	}

	// Ждем штатного завершения ядра (до 2.5 сек для корректного освобождения адаптера Wintun и таблицы маршрутов)
	if coreCmd != nil && coreCmd.Process != nil {
		proc := coreCmd.Process
		select {
		case <-coreDone:
		case <-time.After(2500 * time.Millisecond):
			_ = proc.Kill()
		}
		coreCmd = nil
		coreDone = nil
	}
	isConnected = false
	// The core owns its route list and removes it during the STOP handshake.
	exec.Command("ipconfig", "/flushdns").Run()

	appendLog("[GUI] Atlant VPN core stopped")
}

func streamOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		cmdMu.Lock()
		lvl := "info"
		if activeConfig != nil && activeConfig.LogLevel != "" {
			lvl = activeConfig.LogLevel
		}
		cmdMu.Unlock()
		if lvl == "none" || lvl == "silent" {
			continue
		}
		lower := strings.ToLower(text)
		isError := strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "[err")
		isWarn := strings.Contains(lower, "warn") || strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout")
		isDebug := strings.Contains(lower, "debug")

		switch lvl {
		case "error":
			if isError {
				appendLog(text)
			}
		case "warn", "warning":
			if isError || isWarn {
				appendLog(text)
			}
		case "info":
			if !isDebug {
				appendLog(text)
			}
		case "debug":
			appendLog(text)
		default:
			if !isDebug {
				appendLog(text)
			}
		}
	}
}

func resetNetwork() {
	stopCore()
	// The core cleans its own routes; the GUI restores only the proxy it saved.
	// Never reset Winsock, DHCP, other proxies or another VPN's route table.
	if out, err := exec.Command("ipconfig", "/flushdns").CombinedOutput(); err != nil {
		appendLog(fmt.Sprintf("[NET] DNS cache refresh failed: %v (%s)", err, strings.TrimSpace(string(out))))
		return
	}
	appendLog("[NET] Atlant stopped and DNS cache refreshed.")
}

func parseSubscriptionUserinfo(h http.Header) (upload, download, total, expire int64) {
	userInfo := h.Get("Subscription-Userinfo")
	if userInfo == "" {
		userInfo = h.Get("subscription-userinfo")
	}
	if userInfo != "" {
		parts := strings.Split(userInfo, ";")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			kv := strings.SplitN(p, "=", 2)
			if len(kv) == 2 {
				k := strings.ToLower(strings.TrimSpace(kv[0]))
				v := strings.TrimSpace(kv[1])
				var val int64
				fmt.Sscanf(v, "%d", &val)
				if val < 0 {
					val = 0
				}
				switch k {
				case "upload":
					upload = val
				case "download":
					download = val
				case "total":
					total = val
				case "expire":
					expire = val
				}
			}
		}
	}
	return
}

func parseSubscriptionLinks(body string) []ServerProfile {
	var profiles []ServerProfile
	lines := strings.Split(body, "\n")
	if len(lines) == 1 && len(body) > 40 && !strings.Contains(body, "://") {
		if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(body)); err == nil {
			lines = strings.Split(string(decoded), "\n")
		}
	}

	idx := 1
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "atlant://") {
			if u, err := url.Parse(line); err == nil {
				q := u.Query()
				name := u.Fragment
				if unescaped, err := url.QueryUnescape(name); err == nil && unescaped != "" {
					name = unescaped
				}
				if name == "" {
					name = fmt.Sprintf("Atlant Server #%d", idx)
				}
				idx++
				user := u.User.Username()
				if q.Get("user") != "" {
					user = q.Get("user")
				}
				fp := q.Get("fp")
				if fp == "" {
					fp = q.Get("fingerprint")
				}
				p := ServerProfile{
					ID:           fmt.Sprintf("sub-%d-%d", time.Now().Unix(), idx),
					Name:         name,
					Server:       u.Host,
					SNI:          q.Get("sni"),
					PBK:          q.Get("pbk"),
					ServerPubKey: q.Get("server_pub_key"),
					WireVersion:  parseWireVersion(q.Get("wire_version")),
					User:         user,
					Transport:    q.Get("transport"),
					TrafficMode:  q.Get("traffic_mode"),
					Fingerprint:  fp,
					ServerID:     q.Get("server_id"),
					InboundID:    q.Get("inbound_id"),
					Path:         q.Get("path"),
					Type:         "atlant",
				}
				if p.SNI == "" {
					p.SNI = "www.microsoft.com"
				}
				if p.WireVersion < 0 {
					continue
				}
				var valid bool
				p.Transports, valid = subscriptionTransports(q.Get("transports"))
				if !valid || !validSubscriptionPath(p.Path) {
					continue
				}
				if p.Transport == "" {
					p.Transport = "quic"
				}
				if p.TrafficMode == "" {
					p.TrafficMode = "udp"
				}
				profiles = append(profiles, p)
				if len(profiles) > 256 {
					return nil
				}
			}
		}
	}
	return profiles
}

func launchBrowserWindow(url string) {
	edgePaths := []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	}
	for _, p := range edgePaths {
		if _, err := os.Stat(p); err == nil {
			cmd := exec.Command(p,
				fmt.Sprintf("--app=%s", url),
				"--window-size=840,720",
				"--user-data-dir="+filepath.Join(os.TempDir(), "atlant_vpn_edge_profile"),
				"--enable-gpu-rasterization",
				"--enable-zero-copy",
				"--ignore-gpu-blocklist",
				"--disable-background-timer-throttling",
				"--disable-backgrounding-occluded-windows",
				"--disable-renderer-backgrounding",
			)
			if err := cmd.Start(); err == nil {
				browserCmd = cmd
				return
			}
		}
	}

	chromePath := `C:\Program Files\Google\Chrome\Application\chrome.exe`
	if _, err := os.Stat(chromePath); err == nil {
		cmd := exec.Command(chromePath,
			fmt.Sprintf("--app=%s", url),
			"--window-size=840,720",
			"--enable-gpu-rasterization",
			"--enable-zero-copy",
			"--ignore-gpu-blocklist",
			"--disable-background-timer-throttling",
			"--disable-backgrounding-occluded-windows",
			"--disable-renderer-backgrounding",
		)
		if err := cmd.Start(); err == nil {
			browserCmd = cmd
			return
		}
	}

	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func amAdmin() bool {
	token := windows.GetCurrentProcessToken()
	return token.IsElevated()
}

func runMeElevated() {
	verb := "runas"
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	args := strings.Join(os.Args[1:], " ")

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argsPtr, _ := syscall.UTF16PtrFromString(args)

	var showCmd int32 = 1 // SW_NORMAL
	windows.ShellExecute(0, verbPtr, exePtr, argsPtr, cwdPtr, showCmd)
}

func main() {
	if !amAdmin() {
		runMeElevated()
		return
	}

	activeConfig = loadConfig()
	appendLog("[GUI] Atlant VPN GUI launcher started")
	appendLog(fmt.Sprintf("[GUI] Loaded configuration: %s (SNI: %s, Transport: %s)", activeConfig.Server, activeConfig.SNI, activeConfig.Transport))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to listen on localhost: %v", err)
	}
	addr := listener.Addr().String()

	subFS, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("Failed to sub filesystem: %v", err)
	}

	mux := http.NewServeMux()
	uiDirectory, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("Cannot locate UI preferences directory: %v", err)
	}
	uiPreferencesPath := filepath.Join(uiDirectory, "AtlantVPN", "ui-preferences.json")
	mux.Handle("/api/ui/preferences", newUIPreferencesHandler(uiPreferencesPath))
	mux.HandleFunc("/ui-language.js", uiLanguageScript(uiPreferencesPath))
	fileServer := http.FileServer(http.FS(subFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fileServer.ServeHTTP(w, r)
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		heartbeatMu.Lock()
		lastHeartbeat = time.Now()
		heartbeatMu.Unlock()

		cmdMu.Lock()
		conn := isConnected
		cmdMu.Unlock()

		logMu.Lock()
		logs := strings.Join(logBuffer, "\n")
		logMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"connected": conn,
			"logs":      logs,
		})
	})

	mux.HandleFunc("/api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
		go func() {
			time.Sleep(100 * time.Millisecond)
			stopCore()
			os.Exit(0)
		}()
	})

	mux.HandleFunc("/api/connect", func(w http.ResponseWriter, r *http.Request) {
		err := startCore()
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	})

	mux.HandleFunc("/api/disconnect", func(w http.ResponseWriter, r *http.Request) {
		stopCore()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	})

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(loadConfig())
			return
		}
		if r.Method == http.MethodPost {
			var newCfg AppConfig
			if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			preserveGUICatalog(&newCfg, loadConfig())
			if err := saveConfig(&newCfg); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			cmdMu.Lock()
			activeConfig = &newCfg
			cmdMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
			return
		}
	})

	// Add/Switch server
	mux.HandleFunc("/api/servers/select", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID string `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		cfg := loadConfig()
		for _, s := range cfg.Servers {
			if s.ID == req.ID {
				selectSubscriptionProfile(cfg, s)
				if cfg.TrafficMode == "" && s.TrafficMode != "" {
					cfg.TrafficMode = s.TrafficMode
				}
				if !commitGUIConfig(w, cfg) {
					return
				}
				break
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "config": cfg})
	})

	// Add server
	mux.HandleFunc("/api/servers/add", func(w http.ResponseWriter, r *http.Request) {
		var p ServerProfile
		json.NewDecoder(r.Body).Decode(&p)
		if p.ID == "" {
			p.ID = fmt.Sprintf("srv-%d", time.Now().UnixNano())
		}
		if p.Name == "" {
			p.Name = p.Server
		}
		cfg := loadConfig()
		cfg.Servers = append(cfg.Servers, p)
		if !commitGUIConfig(w, cfg) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "servers": cfg.Servers})
	})

	// Update existing server
	mux.HandleFunc("/api/servers/update", func(w http.ResponseWriter, r *http.Request) {
		var p ServerProfile
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if p.ID == "" {
			http.Error(w, "Server ID is required", 400)
			return
		}
		cfg := loadConfig()
		found := false
		for i, s := range cfg.Servers {
			if s.ID == p.ID {
				// Keep existing ping, group, and type if not provided
				p.Ping = s.Ping
				p.IsSubscription = s.IsSubscription
				p.SubscriptionID = s.SubscriptionID
				p.ServerID, p.InboundID = s.ServerID, s.InboundID
				if s.IsSubscription {
					p.Path, p.Transports = s.Path, s.Transports
				}
				if p.Group == "" && s.Group != "" {
					p.Group = s.Group
				}
				if p.Type == "" && s.Type != "" {
					p.Type = s.Type
				}
				cfg.Servers[i] = p
				found = true
				// If updating active server, also update top-level config
				if cfg.ActiveServerID == p.ID {
					selectSubscriptionProfile(cfg, p)
					if cfg.TrafficMode == "" && p.TrafficMode != "" {
						cfg.TrafficMode = p.TrafficMode
					}
					if p.Security != "" {
						cfg.Security = p.Security
					}
				}
				break
			}
		}
		if !found {
			http.Error(w, "Server not found", 404)
			return
		}
		if !commitGUIConfig(w, cfg) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "servers": cfg.Servers})
	})

	// Paste link or subscription from clipboard
	mux.HandleFunc("/api/servers/paste", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		text := strings.TrimSpace(req.Text)
		if text == "" {
			http.Error(w, "Empty text provided", 400)
			return
		}

		cfg := loadConfig()
		var added []ServerProfile

		// Check if it's an HTTP subscription URL
		if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
			count, err := importSubscriptionURLs(cfg, text)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			if commitGUIConfig(w, cfg) {
				writeSubscriptionResponse(w, cfg, count)
			}
			return
		} else {
			// Try parsing as raw links / JSON
			newServers := parseSubscriptionLinks(text)
			if len(newServers) == 0 {
				// Maybe single link or JSON
				if strings.HasPrefix(text, "{") {
					var p ServerProfile
					if err := json.Unmarshal([]byte(text), &p); err == nil && p.Server != "" {
						if p.ID == "" {
							p.ID = fmt.Sprintf("srv-%d", time.Now().UnixNano())
						}
						if p.Name == "" {
							p.Name = p.Server
						}
						newServers = append(newServers, p)
					}
				}
			}

			existingMap := make(map[string]bool)
			for _, s := range cfg.Servers {
				existingMap[profileIdentity(s)] = true
			}
			for _, ns := range newServers {
				if !existingMap[profileIdentity(ns)] {
					cfg.Servers = append(cfg.Servers, ns)
					added = append(added, ns)
					existingMap[profileIdentity(ns)] = true
				}
			}
		}

		if !commitGUIConfig(w, cfg) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      true,
			"added_count":  len(added),
			"servers":      cfg.Servers,
			"subscription": cfg.Subscription,
		})
	})

	// Delete server
	mux.HandleFunc("/api/servers/delete", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID string `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		cfg := loadConfig()
		var updated []ServerProfile
		for _, s := range cfg.Servers {
			if s.ID != req.ID {
				updated = append(updated, s)
			}
		}
		cfg.Servers = updated
		if !commitGUIConfig(w, cfg) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "servers": cfg.Servers})
	})

	mux.HandleFunc("/api/subscription/update", updateSubscriptionHTTP)
	mux.HandleFunc("/api/subscription/clear", clearSubscriptionHTTP)
	mux.HandleFunc("/api/subscription/edit", editSubscriptionHTTP)

	mux.HandleFunc("/api/ping", newPingHandler(loadConfig, (&net.Dialer{Timeout: 3 * time.Second}).DialContext))

	mux.HandleFunc("/api/reset-network", func(w http.ResponseWriter, r *http.Request) {
		go resetNetwork()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	})

	mux.HandleFunc("/api/logs/clear", func(w http.ResponseWriter, r *http.Request) {
		logMu.Lock()
		logBuffer = nil
		logMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	})

	mux.HandleFunc("/api/apps/running", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		apps := listRunningApps()
		json.NewEncoder(w).Encode(map[string]interface{}{"apps": apps})
	})

	bootstrapToken, err := newGUIBootstrapToken()
	if err != nil {
		log.Fatalf("Cannot initialize GUI bootstrap: %v", err)
	}
	protectedHandler, err := newGUIHandler(addr, mux, bootstrapToken)
	if err != nil {
		log.Fatalf("Cannot initialize GUI session: %v", err)
	}
	server := &http.Server{
		Handler:           protectedHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			appendLog(fmt.Sprintf("[GUI] Server error: %v", err))
		}
	}()

	time.Sleep(150 * time.Millisecond)
	launchBrowserWindow(fmt.Sprintf("http://%s/?bootstrap=%s", addr, bootstrapToken))

	// Keep the GUI HTTP server alive
	select {}
}

func listRunningApps() []string {
	apps := make([]string, 0)
	systemIgnore := map[string]bool{
		"system": true, "system idle process": true, "registry": true, "smss.exe": true,
		"csrss.exe": true, "wininit.exe": true, "services.exe": true, "lsass.exe": true,
		"svchost.exe": true, "fontdrvhost.exe": true, "dwm.exe": true, "tasklist.exe": true,
		"conhost.exe": true, "runtimebroker.exe": true, "sihost.exe": true, "ctfmon.exe": true,
		"atlant_gui.exe": true, "atlant_win_64.exe": true, "cmd.exe": true, "wsl.exe": true,
		"wslhost.exe": true, "searchindexer.exe": true, "searchhost.exe": true,
		"shellexperiencehost.exe": true, "applicationframehost.exe": true,
	}
	seen := make(map[string]bool)

	cmd := exec.Command("tasklist", "/fo", "csv", "/nh")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, ",")
			if len(parts) > 0 {
				name := strings.Trim(parts[0], "\" \r\n")
				lower := strings.ToLower(name)
				if strings.HasSuffix(lower, ".exe") && !systemIgnore[lower] && !seen[lower] {
					seen[lower] = true
					apps = append(apps, name)
				}
			}
		}
	}

	if len(apps) == 0 {
		psCmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "Get-Process | Where-Object { $_.MainWindowTitle -or $_.ProcessName } | Select-Object -ExpandProperty ProcessName -Unique")
		psCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if psOut, psErr := psCmd.Output(); psErr == nil {
			for _, line := range strings.Split(string(psOut), "\n") {
				name := strings.TrimSpace(line)
				if name == "" {
					continue
				}
				exeName := name + ".exe"
				lower := strings.ToLower(exeName)
				if !systemIgnore[lower] && !seen[lower] {
					seen[lower] = true
					apps = append(apps, exeName)
				}
			}
		}
	}

	sort.Strings(apps)
	return apps
}
