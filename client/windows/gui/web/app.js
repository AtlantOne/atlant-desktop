// Atlant VPN Web Client Logic

let isConnected = false;
let isConnecting = false;
let pollTimer = null;
let speedTimer = null;
let editingServerId = null;

// Theme initialization is isolated in theme.js; unavailable storage must not break the app.

// DOM Elements
const navItems = document.querySelectorAll('.nav-item');
const tabPanes = document.querySelectorAll('.tab-pane');
const connectBtn = document.getElementById('btn-connect');
const connectSection = document.querySelector('.connect-section');
const connectLabel = document.getElementById('connect-label');
const statusText = document.getElementById('status-text');
const pingValue = document.getElementById('ping-value');
const pingBtn = document.getElementById('btn-ping');
const logOutput = document.getElementById('log-output');

// Config Elements
const checkTun = document.getElementById('check-tun');
const checkKillswitch = document.getElementById('check-killswitch');

// Per-App Routing State
let currentRouteMode = 'all';
let proxyAppsList = [];
let bypassAppsList = [];
let activeAppPickerTarget = 'proxy'; // 'proxy' or 'bypass'
let cachedRunningApps = [];
let currentActiveTab = 'tab-dashboard';

// Tab Navigation
function switchToTab(tabId) {
  currentActiveTab = tabId;
  navItems.forEach(n => {
    if (n.getAttribute('data-tab') === tabId) {
      n.classList.add('active');
    } else {
      n.classList.remove('active');
    }
  });
  tabPanes.forEach(p => {
    if (p.id === tabId) {
      p.classList.add('active');
    } else {
      p.classList.remove('active');
    }
  });
  if (tabId === 'tab-logs') {
    if (logsNeedRender) {
      logsNeedRender = false;
      renderLogsDOM(cachedLogsRaw);
    }
  }
}

navItems.forEach(item => {
  item.addEventListener('click', () => {
    const tabId = item.getAttribute('data-tab');
    switchToTab(tabId);
  });
});

const btnOpenServerList = document.getElementById('btn-open-server-list');
if (btnOpenServerList) {
  btnOpenServerList.addEventListener('keydown', (event) => {
    if (event.target === btnOpenServerList && (event.key === 'Enter' || event.key === ' ')) {
      event.preventDefault(); switchToTab('tab-servers');
    }
  });
  btnOpenServerList.addEventListener('click', () => {
    switchToTab('tab-servers');
  });
}

// Toast notification
function showToast(msg) {
  const toast = document.getElementById('toast');
  if (!toast) return;
  toast.textContent = msg;
  toast.classList.add('show');
  setTimeout(() => toast.classList.remove('show'), 2600);
}

// Connect / Disconnect Toggle
if (connectBtn) {
  connectBtn.addEventListener('click', async () => {
    if (isConnecting) return;
    if (isConnected) {
      await stopVPN();
    } else {
      await startVPN();
    }
  });
}

async function startVPN() {
  setConnectingState(true);
  try {
    const res = await fetch('/api/connect', { method: 'POST' });
    const data = await res.json();
    if (!data.success) {
      showToast(tr("Ошибка запуска VPN: ") + (data.error || tr("Неизвестная ошибка")));
      setConnectingState(false);
      setConnectedState(false);
    }
  } catch (err) {
    showToast(tr("Ошибка связи с ядром: ") + err.message);
    setConnectingState(false);
    setConnectedState(false);
  }
}

async function stopVPN() {
  try {
    await fetch('/api/disconnect', { method: 'POST' });
  } catch (e) {}
  setConnectedState(false);
  setConnectingState(false);
}

function setConnectingState(connecting) {
  isConnecting = connecting;
  if (connecting) {
    if (connectSection) connectSection.className = 'connect-section connecting';
    if (connectLabel) connectLabel.textContent = tr("ПОДКЛЮЧЕНИЕ");
    if (statusText) statusText.textContent = tr("Устанавливаем соединение…");
    if (connectBtn) { connectBtn.setAttribute('aria-busy', 'true'); connectBtn.disabled = true; }
    const caption = document.getElementById('connection-caption');
    if (caption) caption.textContent = tr("Ожидаем ответ ядра VPN.");
  }
}

function setConnectedState(connected) {
  isConnected = connected;
  isConnecting = false;
  if (connectBtn) {
    connectBtn.disabled = false;
    connectBtn.setAttribute('aria-busy', 'false');
    connectBtn.setAttribute('aria-pressed', String(connected));
    connectBtn.setAttribute('aria-label', connected ? tr("Отключить VPN") : tr("Подключить VPN"));
  }
  const caption = document.getElementById('connection-caption');
  if (caption) caption.textContent = connected ? tr("Ядро запущено. Доступность интернета проверяется отдельно.") : tr("Выберите сервер и подключитесь, когда будете готовы.");
  if (connected) {
    if (connectSection) connectSection.className = 'connect-section connected';
    if (connectLabel) connectLabel.textContent = tr("ОТКЛЮЧИТЬ");
    if (statusText) statusText.textContent = tr("Туннель запущен");
  } else {
    if (connectSection) connectSection.className = 'connect-section';
    if (connectLabel) connectLabel.textContent = tr("ПОДКЛЮЧИТЬ");
    if (statusText) statusText.textContent = tr("Соединение отключено");
  }
}

// Ping Checker
if (pingBtn) {
  pingBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    checkPing();
  });
}
async function checkPing(serverAddr) {
  if (pingValue && !serverAddr) pingValue.textContent = '...';
  const url = serverAddr ? ('/api/ping?server=' + encodeURIComponent(serverAddr)) : '/api/ping';
  try {
    const res = await fetch(url);
    const data = await res.json();
    if (res.ok && Number.isFinite(data.rtt) && data.rtt >= 0) {
      if (!serverAddr && pingValue) {
        pingValue.textContent = data.rtt + ' ms';
        pingValue.style.color = data.rtt < 100 ? '#10b981' : '#f59e0b';
        pingValue.title = tr("Время TCP-подключения; не проверка VPN-аутентификации");
      }
      return data.rtt;
    } else {
      if (!serverAddr && pingValue) {
        pingValue.textContent = data.error_code === 'no_server' ? tr("Выберите сервер") : tr("Нет TCP");
        pingValue.title = data.error || tr("TCP-подключение не удалось");
        pingValue.style.color = '#f43f5e';
        if (data.error) showToast(data.error);
      }
      return -1;
    }
  } catch (e) {
    if (!serverAddr && pingValue) {
      pingValue.textContent = tr("Ошибка проверки");
      pingValue.title = tr("Не удалось получить ответ GUI: ") + String(e.message || e);
    }
    return -1;
  }
}

// Reset Network
const btnResetNet = document.getElementById('btn-reset-network');
if (btnResetNet) {
  btnResetNet.addEventListener('click', async () => {
    if (confirm(tr("Сбросить сетевые маршруты, кэш DNS и Winsock до стандартных?"))) {
      showToast(tr("Сброс сети Windows..."));
      await fetch('/api/reset-network', { method: 'POST' });
      showToast(tr("Сброс сети выполнен"));
    }
  });
}

// Load Config from Backend
let globalConfig = null;

function selectStoredFingerprint(select, value) {
  let profile = typeof value === 'string' && value ? value : 'firefox';
  if (['chrome_144', 'chrome_133', 'chrome_auto'].includes(profile.trim().toLowerCase())) profile = 'chrome';
  select.querySelectorAll('[data-imported-profile]').forEach(option => option.remove());
  if (!Array.from(select.options).some(option => option.value === profile)) {
    const option = document.createElement('option');
    option.value = profile;
    option.textContent = tr("{0} (импортирован; проверьте совместимость)", profile);
    option.dataset.importedProfile = 'true';
    select.appendChild(option);
  }
  select.value = profile;
}

async function loadConfig() {
  try {
    const res = await fetch('/api/config');
    const cfg = await res.json();
    globalConfig = cfg;

    if (checkTun) checkTun.checked = !!cfg.tun;
    if (checkKillswitch) checkKillswitch.checked = !!cfg.kill_switch;

    // Populate Settings tab inputs
    const setMtu = document.getElementById('set-mtu');
    const setDns = document.getElementById('set-dns');
    const setTun = document.getElementById('set-tun');
    const setKillswitch = document.getElementById('set-killswitch');
    const setBlockIpv6 = document.getElementById('set-block-ipv6');
    const setTrafficMode = document.getElementById('set-traffic-mode');
    const setUdpProto = document.getElementById('set-udp-proto');
    const setTcpProto = document.getElementById('set-tcp-proto');
    const setTcpOnlyProto = document.getElementById('set-tcp-only-proto');
    const setUdpOnlyProto = document.getElementById('set-udp-only-proto');
    const setLocalPort = document.getElementById('set-local-port');
    const setAllowLan = document.getElementById('set-allow-lan');
    const setMux = document.getElementById('set-mux');

    if (setMtu) setMtu.value = cfg.mtu || 1360;
    if (setDns) setDns.value = cfg.dns || '1.1.1.1';
    if (setTun) setTun.checked = !!cfg.tun;
    if (setKillswitch) setKillswitch.checked = !!cfg.kill_switch;
    if (setBlockIpv6) setBlockIpv6.checked = !!cfg.block_ipv6;
    if (setTrafficMode) setTrafficMode.value = cfg.traffic_mode || 'split';
    if (setUdpProto) setUdpProto.value = cfg.udp_proto === 'tcp' ? 'tcp' : 'quic';
    if (setTcpProto) setTcpProto.value = cfg.tcp_proto === 'xhttp' ? 'xhttp' : 'reality';
    if (setLocalPort) setLocalPort.value = cfg.local || '127.0.0.1:1080';
    if (setAllowLan) setAllowLan.checked = !!cfg.allow_lan;
    const setLogLevel = document.getElementById('set-log-level');
    if (setLogLevel) setLogLevel.value = cfg.log_level || 'info';

    // Anti-DPI & Protocol Optimization
    const setFakeDns = document.getElementById('set-fake-dns');
    const setTrafficPadding = document.getElementById('set-traffic-padding');
    const setAutoFallback = document.getElementById('set-auto-fallback');
    const setPortHopping = document.getElementById('set-port-hopping');
    const setRemoteDns = document.getElementById('set-remote-dns');
    const setDecoyTraffic = document.getElementById('set-decoy-traffic');
    const setZeroRtt = document.getElementById('set-zero-rtt');

    if (setFakeDns) setFakeDns.checked = cfg.fake_dns !== false;
    if (setTrafficPadding) setTrafficPadding.checked = cfg.traffic_padding !== false;
    if (setAutoFallback) setAutoFallback.checked = cfg.auto_fallback !== false;
    if (setPortHopping) setPortHopping.checked = !!cfg.port_hopping;
    if (setRemoteDns) setRemoteDns.checked = cfg.remote_dns !== false;
    if (setDecoyTraffic) setDecoyTraffic.checked = !!cfg.decoy_traffic;
    if (setZeroRtt) setZeroRtt.checked = cfg.zero_rtt !== false;

    // uTLS, Traffic Padding & Chaffing settings
    const setFingerprint = document.getElementById('set-fingerprint');
    const setPaddingEnabled = document.getElementById('set-padding-enabled');
    const setMinPadding = document.getElementById('set-min-padding');
    const setMaxPadding = document.getElementById('set-max-padding');
    const setChaffingEnabled = document.getElementById('set-chaffing-enabled');
    const setChaffingInterval = document.getElementById('set-chaffing-interval');

    if (setFingerprint) selectStoredFingerprint(setFingerprint, cfg.fingerprint);
    if (setPaddingEnabled) setPaddingEnabled.checked = cfg.padding_enabled !== false;
    if (setMinPadding) setMinPadding.value = cfg.min_padding || 32;
    if (setMaxPadding) setMaxPadding.value = cfg.max_padding || 256;
    if (setChaffingEnabled) setChaffingEnabled.checked = cfg.chaffing_enabled !== false;
    if (setChaffingInterval) setChaffingInterval.value = cfg.chaffing_interval || 10;

    updateTrafficModeUI();

    // Per-App & Domain Route Mode & Lists
    currentRouteMode = cfg.route_mode || 'all';
    if (currentRouteMode === 'proxy_apps') currentRouteMode = 'rules';
    if (currentRouteMode === 'bypass_apps') currentRouteMode = 'bypass';

    proxyAppsList = Array.isArray(cfg.proxy_apps) ? [...cfg.proxy_apps] : [];
    bypassAppsList = Array.isArray(cfg.bypass_apps) ? [...cfg.bypass_apps] : [];

    const proxyDomainsEl = document.getElementById('cfg-proxy-domains');
    const directDomainsEl = document.getElementById('cfg-direct-domains');
    if (proxyDomainsEl) proxyDomainsEl.value = (cfg.proxy_domains || []).join('\n');
    if (directDomainsEl) directDomainsEl.value = (cfg.direct_domains || []).join('\n');
    updateDomainCounters();

    selectRouteMode(currentRouteMode, false);
    renderAppsLists();

    // Update Dashboard Card
    const topName = document.getElementById('card-server-name');
    const topMeta = document.getElementById('card-server-meta');
    let activeName = cfg.server || tr("Добавьте первый сервер");
    let activeMeta = cfg.server ? tr("Профиль Atlant") : tr("Вставьте ссылку или импортируйте подписку");
    let activeTransport = cfg.server ? (cfg.transport || 'reality') : tr("Не выбран");
    if (cfg.servers && cfg.servers.length > 0) {
      const activeSrv = cfg.servers.find(s => s.id === cfg.active_server_id) || cfg.servers[0];
      if (activeSrv) {
        activeName = activeSrv.name || activeSrv.server;
        activeTransport = activeSrv.transport || cfg.transport || 'reality';
        activeMeta = `${activeSrv.server} · ${activeTransport.toUpperCase()}`;
      }
    }
    if (topName) topName.textContent = activeName;
    if (topMeta) topMeta.textContent = activeMeta;
    const transportFact = document.getElementById('dashboard-transport');
    const modeFact = document.getElementById('dashboard-mode');
    if (transportFact) transportFact.textContent = activeTransport.toUpperCase();
    if (modeFact) modeFact.textContent = cfg.tun ? tr("Системный · TUN") : tr("Локальный прокси");

    // Render Grouped Providers & Servers List (Screenshots layout)
    renderGroupedProviders(cfg.servers || [], cfg.active_server_id, cfg.subscriptions || cfg.subscription);
  } catch (e) {
    console.error('Failed to load config:', e);
  }
}

// Update Traffic Mode visibility (Sub-box appears ONLY in Split mode, completely hidden otherwise)
function updateTrafficModeUI() {
  const setTrafficMode = document.getElementById('set-traffic-mode');
  const splitProtocolDetails = document.getElementById('split-protocol-details');
  if (!setTrafficMode || !splitProtocolDetails) return;

  const mode = setTrafficMode.value;
  if (mode !== 'udp') {
    splitProtocolDetails.style.display = 'flex';
  } else {
    splitProtocolDetails.style.display = 'none';
  }
  const udpRow = document.getElementById('set-udp-proto')?.closest('.settings-row');
  if (udpRow) udpRow.hidden = mode !== 'split';
}

const setTrafficModeEl = document.getElementById('set-traffic-mode');
if (setTrafficModeEl) {
  setTrafficModeEl.addEventListener('change', updateTrafficModeUI);
}

// Save Config
let configWriteQueue = Promise.resolve();
function commitConfig(patch) {
  const task = configWriteQueue.then(async () => {
    const cfg = { ...globalConfig, ...patch };
    const res = await fetch('/api/config', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(cfg) });
    if (!res.ok) throw new Error(tr("Сервер не принял настройки"));
    globalConfig = cfg;
    return cfg;
  });
  configWriteQueue = task.catch(() => {});
  return task;
}

async function saveConfig(silent = false) {
  if (!globalConfig) {
    try {
      const res = await fetch('/api/config');
      if (!res.ok) throw new Error('Config load failed');
      globalConfig = await res.json();
    } catch (e) { showToast(tr("Не удалось загрузить настройки. Изменения не сохранены.")); return false; }
  }

  const proxyDomainsEl = document.getElementById('cfg-proxy-domains');
  const directDomainsEl = document.getElementById('cfg-direct-domains');

  const cfg = {
    route_mode: currentRouteMode,
    proxy_apps: proxyAppsList,
    bypass_apps: bypassAppsList,
    proxy_domains: proxyDomainsEl ? proxyDomainsEl.value.split('\n').map(s => s.trim()).filter(Boolean) : (globalConfig && globalConfig.proxy_domains || []),
    direct_domains: directDomainsEl ? directDomainsEl.value.split('\n').map(s => s.trim()).filter(Boolean) : (globalConfig && globalConfig.direct_domains || []),
    tun: checkTun ? checkTun.checked : (globalConfig ? globalConfig.tun : false),
    kill_switch: checkKillswitch ? checkKillswitch.checked : (globalConfig ? globalConfig.kill_switch : false),
  };

  try {
    await commitConfig(cfg);
    if (!silent) showToast(tr("Правила сохранены. Для применения переподключите VPN."));
    return true;
  } catch (e) {
    showToast(tr("Ошибка сохранения: ") + e.message);
    return false;
  }
}

// TUN & KillSwitch toggle listeners
if (checkTun) {
  checkTun.addEventListener('change', async () => {
    const saved = await saveConfig();
    if (saved && isConnected) {
      showToast(tr("Перезапуск VPN для применения TUN режима..."));
      await stopVPN();
      setTimeout(startVPN, 500);
    }
  });
}

if (checkKillswitch) {
  checkKillswitch.addEventListener('change', async () => {
    const saved = await saveConfig();
    if (saved && isConnected) {
      showToast(tr("Перезапуск VPN для применения Kill Switch..."));
      await stopVPN();
      setTimeout(startVPN, 500);
    }
  });
}

const btnSaveRouting = document.getElementById('btn-save-routing');
if (btnSaveRouting) {
  btnSaveRouting.addEventListener('click', async () => {
    btnSaveRouting.disabled = true;
    btnSaveRouting.textContent = tr("Сохранение...");
    try { await saveConfig(); }
    finally { btnSaveRouting.textContent = tr("Сохранить правила"); btnSaveRouting.disabled = false; }
  });
}

// Сохранение общих настроек и применение пресетов.
const btnSaveSettings = document.getElementById('btn-save-settings');
if (btnSaveSettings) {
  btnSaveSettings.addEventListener('click', async () => {
    btnSaveSettings.disabled = true;
    btnSaveSettings.textContent = tr("Сохранение...");
    try { await saveGlobalSettings(); }
    finally { btnSaveSettings.textContent = tr("Сохранить настройки"); btnSaveSettings.disabled = false; }
  });
}

async function saveGlobalSettings(silent = false) {
  if (!globalConfig) {
    try {
      const res = await fetch('/api/config');
      if (!res.ok) throw new Error('Config load failed');
      globalConfig = await res.json();
    } catch (e) { showToast(tr("Не удалось загрузить настройки. Изменения не сохранены.")); return false; }
  }

  const setMtu = document.getElementById('set-mtu');
  const setDns = document.getElementById('set-dns');
  const setTun = document.getElementById('set-tun');
  const setKillswitch = document.getElementById('set-killswitch');
  const setBlockIpv6 = document.getElementById('set-block-ipv6');
  const setTrafficMode = document.getElementById('set-traffic-mode');
  const setUdpProto = document.getElementById('set-udp-proto');
  const setTcpProto = document.getElementById('set-tcp-proto');
  const setLocalPort = document.getElementById('set-local-port');
  const setAllowLan = document.getElementById('set-allow-lan');
  const setMux = document.getElementById('set-mux');
  const setLogLevel = document.getElementById('set-log-level');

  const mtuVal = setMtu ? Number(setMtu.value) : 1360;
  const dnsVal = setDns ? setDns.value.trim() : '1.1.1.1';
  if (!Number.isInteger(mtuVal) || mtuVal < 1200 || mtuVal > 1500 || !dnsVal) {
    showToast(tr("Укажите MTU от 1200 до 1500 и адрес DNS-сервера."));
    return false;
  }
  const tunVal = setTun ? setTun.checked : (globalConfig ? globalConfig.tun : false);
  const ksVal = setKillswitch ? setKillswitch.checked : (globalConfig ? globalConfig.kill_switch : false);
  const blockIpv6Val = setBlockIpv6 ? setBlockIpv6.checked : false;
  const tModeVal = setTrafficMode ? setTrafficMode.value : 'split';
  const muxVal = setMux ? setMux.checked : false;
  const logLevelVal = setLogLevel ? setLogLevel.value : 'info';

  let udpProtoVal = 'quic';
  let tcpProtoVal = 'reality';

  if (tModeVal === 'split') {
    udpProtoVal = setUdpProto ? setUdpProto.value : 'quic';
    tcpProtoVal = setTcpProto ? setTcpProto.value : 'reality';
  } else if (tModeVal === 'tcp') {
    udpProtoVal = 'tcp';
    tcpProtoVal = setTcpProto ? setTcpProto.value : 'reality';
  } else if (tModeVal === 'udp') {
    udpProtoVal = 'quic';
    tcpProtoVal = 'udp';
  }

  const transportVal = tModeVal === 'udp' ? 'quic' : tcpProtoVal === 'xhttp' ? 'xhttp' : 'reality';

  const lPortVal = setLocalPort ? setLocalPort.value.trim() || '127.0.0.1:1080' : '127.0.0.1:1080';
  const allowLanVal = setAllowLan ? setAllowLan.checked : false;

  const setFakeDns = document.getElementById('set-fake-dns');
  const setTrafficPadding = document.getElementById('set-traffic-padding');
  const setAutoFallback = document.getElementById('set-auto-fallback');
  const setPortHopping = document.getElementById('set-port-hopping');
  const setRemoteDns = document.getElementById('set-remote-dns');
  const setDecoyTraffic = document.getElementById('set-decoy-traffic');
  const setZeroRtt = document.getElementById('set-zero-rtt');

  const fakeDnsVal = setFakeDns ? setFakeDns.checked : true;
  const trafficPaddingVal = setTrafficPadding ? setTrafficPadding.checked : true;
  const autoFallbackVal = setAutoFallback ? setAutoFallback.checked : true;
  const portHoppingVal = setPortHopping ? setPortHopping.checked : false;
  const remoteDnsVal = setRemoteDns ? setRemoteDns.checked : true;
  const decoyTrafficVal = setDecoyTraffic ? setDecoyTraffic.checked : false;
  const zeroRttVal = setZeroRtt ? setZeroRtt.checked : true;

  const proxyDomainsEl = document.getElementById('cfg-proxy-domains');
  const directDomainsEl = document.getElementById('cfg-direct-domains');
  const proxyDoms = proxyDomainsEl ? proxyDomainsEl.value.split('\n').map(s => s.trim()).filter(Boolean) : (globalConfig?.proxy_domains || []);
  const directDoms = directDomainsEl ? directDomainsEl.value.split('\n').map(s => s.trim()).filter(Boolean) : (globalConfig?.direct_domains || []);

  const cfg = {
    route_mode: currentRouteMode,
    proxy_apps: proxyAppsList,
    bypass_apps: bypassAppsList,
    proxy_domains: proxyDoms,
    direct_domains: directDoms,
    mtu: mtuVal,
    dns: dnsVal,
    tun: tunVal,
    kill_switch: ksVal,
    block_ipv6: blockIpv6Val,
    traffic_mode: tModeVal,
    transport: transportVal,
    udp_proto: udpProtoVal,
    tcp_proto: tcpProtoVal,
    local: lPortVal,
    allow_lan: allowLanVal,
    mux: muxVal,
    log_level: logLevelVal,
    fake_dns: fakeDnsVal,
    traffic_padding: trafficPaddingVal,
    auto_fallback: autoFallbackVal,
    port_hopping: portHoppingVal,
    remote_dns: remoteDnsVal,
    decoy_traffic: decoyTrafficVal,
    zero_rtt: zeroRttVal,
    fingerprint: document.getElementById('set-fingerprint') ? document.getElementById('set-fingerprint').value : 'firefox',
    padding_enabled: document.getElementById('set-padding-enabled') ? document.getElementById('set-padding-enabled').checked : true,
    min_padding: document.getElementById('set-min-padding') ? Number(document.getElementById('set-min-padding').value) : 32,
    max_padding: document.getElementById('set-max-padding') ? parseInt(document.getElementById('set-max-padding').value) || 256 : 256,
    chaffing_enabled: document.getElementById('set-chaffing-enabled') ? document.getElementById('set-chaffing-enabled').checked : true,
    chaffing_interval: document.getElementById('set-chaffing-interval') ? parseInt(document.getElementById('set-chaffing-interval').value) || 10 : 10,
    chaffing_min_bytes: globalConfig?.chaffing_min_bytes ?? 16,
    chaffing_max_bytes: globalConfig?.chaffing_max_bytes ?? 64
  };

  try {
    await commitConfig(cfg);
      if (checkTun) checkTun.checked = tunVal;
      if (checkKillswitch) checkKillswitch.checked = ksVal;
      if (!silent) showToast(tr("Настройки сохранены"));
      if (isConnected && !silent) {
        showToast(tr("Перезапуск VPN для применения настроек..."));
        await stopVPN();
        setTimeout(startVPN, 500);
      }
    return true;
  } catch (e) {
    showToast(tr("Сбой сохранения: ") + e.message);
    return false;
  }
}



const btnSettingsResetNet = document.getElementById('btn-settings-reset-net');
if (btnSettingsResetNet) {
  btnSettingsResetNet.addEventListener('click', async () => {
    if (confirm(tr("Сбросить сетевые маршруты, кэш DNS и Winsock до стандартных?"))) {
      showToast(tr("Сброс сети Windows..."));
      await fetch('/api/reset-network', { method: 'POST' });
      showToast(tr("Сброс сети выполнен успешно"));
    }
  });
}

// Маршрутизация приложений, доменов и IP-адресов.

function updateDomainCounters() {
  const pEl = document.getElementById('cfg-proxy-domains');
  const dEl = document.getElementById('cfg-direct-domains');
  const pCount = document.getElementById('count-proxy-domains');
  const dCount = document.getElementById('count-direct-domains');
  if (pEl && pCount) {
    const lines = pEl.value.split('\n').map(s => s.trim()).filter(Boolean);
    pCount.textContent = `${lines.length}`;
  }
  if (dEl && dCount) {
    const lines = dEl.value.split('\n').map(s => s.trim()).filter(Boolean);
    dCount.textContent = `${lines.length}`;
  }
}

const pDomInput = document.getElementById('cfg-proxy-domains');
if (pDomInput) {
  pDomInput.addEventListener('input', updateDomainCounters);
  pDomInput.addEventListener('change', () => saveConfig(true));
  pDomInput.addEventListener('blur', () => saveConfig(true));
}
const dDomInput = document.getElementById('cfg-direct-domains');
if (dDomInput) {
  dDomInput.addEventListener('input', updateDomainCounters);
  dDomInput.addEventListener('change', () => saveConfig(true));
  dDomInput.addEventListener('blur', () => saveConfig(true));
}

function selectRouteMode(mode, autoSave = false) {
  currentRouteMode = mode;

  const isAll = (mode === 'all');
  const isRules = (mode === 'rules' || mode === 'proxy_apps');
  const isBypass = (mode === 'bypass' || mode === 'bypass_apps');

  // Toggle button active classes
  const btnAll = document.getElementById('btn-mode-all');
  const btnProxy = document.getElementById('btn-mode-proxy-apps');
  const btnBypass = document.getElementById('btn-mode-bypass-apps');

  if (btnAll) btnAll.classList.toggle('active', isAll);
  if (btnProxy) btnProxy.classList.toggle('active', isRules);
  if (btnBypass) btnBypass.classList.toggle('active', isBypass);

  // Toggle dynamic panels (В глобальном режиме пусто!)
  const panelAll = document.getElementById('panel-mode-all');
  const panelProxy = document.getElementById('panel-mode-proxy-apps');
  const panelBypass = document.getElementById('panel-mode-bypass-apps');

  if (panelAll) panelAll.style.display = isAll ? 'block' : 'none';
  if (panelProxy) panelProxy.style.display = isRules ? 'flex' : 'none';
  if (panelBypass) panelBypass.style.display = isBypass ? 'flex' : 'none';
  const viewPicker = document.querySelector('.routing-view-picker');
  if (viewPicker) viewPicker.hidden = isAll;
  [btnAll, btnProxy, btnBypass].forEach(button => {
    if (button) button.setAttribute('aria-pressed', String(button.classList.contains('active')));
  });

  if (autoSave) {
    saveConfig();
  }
}

// Mode Buttons Click Handlers
const btnModeAll = document.getElementById('btn-mode-all');
if (btnModeAll) {
  btnModeAll.addEventListener('click', () => selectRouteMode('all', true));
}

const btnModeProxy = document.getElementById('btn-mode-proxy-apps');
if (btnModeProxy) {
  btnModeProxy.addEventListener('click', () => selectRouteMode('rules', true));
}

const btnModeBypass = document.getElementById('btn-mode-bypass-apps');
if (btnModeBypass) {
  btnModeBypass.addEventListener('click', () => selectRouteMode('bypass', true));
}

// Render Apps Lists
function renderAppsLists() {
  renderSingleAppList('proxy_apps', proxyAppsList, 'list-proxy-apps', 'count-proxy-apps', tr("Список пуст. Добавьте приложения кнопкой «Выбрать .exe» или «Запущенные»."));
  renderSingleAppList('bypass_apps', bypassAppsList, 'list-bypass-apps', 'count-bypass-apps', tr("Список исключений пуст. Добавьте приложения для прямого доступа."));
}

function renderSingleAppList(modeType, list, containerId, countBadgeId, emptyText) {
  const container = document.getElementById(containerId);
  const countBadge = document.getElementById(countBadgeId);

  if (countBadge) {
    countBadge.textContent = `${list.length}`;
  }

  if (!container) return;
  container.innerHTML = '';

  if (list.length === 0) {
    const emptyEl = document.createElement('div');
    emptyEl.className = 'apps-empty-state';
    emptyEl.textContent = emptyText;
    container.appendChild(emptyEl);
    return;
  }

  list.forEach((appName, index) => {
    const item = document.createElement('div');
    item.className = 'app-item';

    item.innerHTML = `
      <div class="app-item-left">
        <div class="app-item-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" width="16" height="16">
            <rect x="3" y="3" width="18" height="18" rx="3" ry="3"/>
            <line x1="8" y1="12" x2="16" y2="12"/>
            <line x1="12" y1="8" x2="12" y2="16"/>
          </svg>
        </div>
        <div class="app-item-info">
          <div class="app-item-name">${escapeHtml(appName)}</div>
        </div>
      </div>
      <button type="button" class="btn-remove-app" title="${tr("Удалить из списка")}" data-index="${index}">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="15" height="15">
          <line x1="18" y1="6" x2="6" y2="18"/>
          <line x1="6" y1="6" x2="18" y2="18"/>
        </svg>
      </button>
    `;

    const btnRemove = item.querySelector('.btn-remove-app');
    if (btnRemove) {
      btnRemove.addEventListener('click', (e) => {
        e.stopPropagation();
        removeAppFromList(modeType, index);
      });
    }

    container.appendChild(item);
  });
}

function addAppToList(modeType, name) {
  let cleanName = (name || '').trim();
  if (!cleanName) return false;

  // Ensure .exe if no extension
  if (!cleanName.includes('.')) {
    cleanName += '.exe';
  }

  const targetList = (modeType === 'proxy_apps' || modeType === 'proxy') ? proxyAppsList : bypassAppsList;
  const lower = cleanName.toLowerCase();

  if (targetList.some(item => item.toLowerCase() === lower)) {
    showToast(tr("Приложение {0} уже есть в списке", cleanName));
    return false;
  }

  targetList.push(cleanName);
  renderAppsLists();
  saveConfig(true);
  showToast(tr("Добавлено: {0}", cleanName));
  return true;
}

function removeAppFromList(modeType, index) {
  const targetList = (modeType === 'proxy_apps' || modeType === 'proxy') ? proxyAppsList : bypassAppsList;
  if (index >= 0 && index < targetList.length) {
    const removed = targetList.splice(index, 1);
    renderAppsLists();
    saveConfig(true);
    showToast(tr("Удалено: {0}", removed[0]));
  }
}

// File Picker Integration
const appFileInput = document.getElementById('input-file-app-picker');
const btnPickFileProxy = document.getElementById('btn-pick-file-proxy');
const btnPickFileBypass = document.getElementById('btn-pick-file-bypass');

if (btnPickFileProxy && appFileInput) {
  btnPickFileProxy.addEventListener('click', () => {
    activeAppPickerTarget = 'proxy_apps';
    appFileInput.value = '';
    appFileInput.click();
  });
}

if (btnPickFileBypass && appFileInput) {
  btnPickFileBypass.addEventListener('click', () => {
    activeAppPickerTarget = 'bypass_apps';
    appFileInput.value = '';
    appFileInput.click();
  });
}

if (appFileInput) {
  appFileInput.addEventListener('change', () => {
    if (appFileInput.files && appFileInput.files.length > 0) {
      const file = appFileInput.files[0];
      addAppToList(activeAppPickerTarget, file.name);
    }
  });
}



// Running Processes Modal
const modalRunningApps = document.getElementById('modal-running-apps');
const btnCloseRunningModal = document.getElementById('btn-close-running-modal');
const btnCancelRunningModal = document.getElementById('btn-cancel-running-modal');
const btnPickRunningProxy = document.getElementById('btn-pick-running-proxy');
const btnPickRunningBypass = document.getElementById('btn-pick-running-bypass');
const runningAppsContainer = document.getElementById('running-apps-items');
const inputSearchRunning = document.getElementById('input-search-running-apps');

function openRunningAppsModal(targetMode) {
  activeAppPickerTarget = targetMode;
  if (modalRunningApps) {
    modalRunningApps.classList.add('open');
    modalRunningApps.style.display = 'flex';
  }
  if (inputSearchRunning) {
    inputSearchRunning.value = '';
    setTimeout(() => inputSearchRunning.focus(), 50);
  }
  loadRunningAppsList();
}

function closeRunningAppsModal() {
  if (modalRunningApps) {
    modalRunningApps.classList.remove('open');
    modalRunningApps.style.display = 'none';
  }
}

if (btnPickRunningProxy) {
  btnPickRunningProxy.addEventListener('click', () => openRunningAppsModal('proxy_apps'));
}
if (btnPickRunningBypass) {
  btnPickRunningBypass.addEventListener('click', () => openRunningAppsModal('bypass_apps'));
}
if (btnCloseRunningModal) {
  btnCloseRunningModal.addEventListener('click', closeRunningAppsModal);
}
if (btnCancelRunningModal) {
  btnCancelRunningModal.addEventListener('click', closeRunningAppsModal);
}
if (modalRunningApps) {
  modalRunningApps.addEventListener('click', (e) => {
    if (e.target === modalRunningApps) closeRunningAppsModal();
  });
}

async function loadRunningAppsList() {
  if (!runningAppsContainer) return;
  runningAppsContainer.innerHTML = `<div class="running-apps-loading">${tr("Загрузка процессов Windows...")}</div>`;

  try {
    const res = await fetch('/api/apps/running');
    const data = await res.json();
    cachedRunningApps = Array.isArray(data.apps) ? data.apps : [];
    renderRunningAppsList(cachedRunningApps);
  } catch (err) {
    runningAppsContainer.innerHTML = `<div class="running-apps-loading" style="color:var(--danger)">${tr("Не удалось загрузить процессы: {0}", escapeHtml(err.message))}</div>`;
  }
}

function renderRunningAppsList(apps) {
  if (!runningAppsContainer) return;
  runningAppsContainer.innerHTML = '';

  const query = (inputSearchRunning ? inputSearchRunning.value : '').toLowerCase().trim();
  const filtered = apps.filter(name => !query || name.toLowerCase().includes(query));

  if (filtered.length === 0) {
    runningAppsContainer.innerHTML = `<div class="running-apps-loading">${tr("Процессы не найдены")}</div>`;
    return;
  }

  filtered.forEach(appName => {
    const row = document.createElement('div');
    row.className = 'running-app-row';

    row.innerHTML = `
      <div class="running-app-info">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" width="16" height="16" style="color:var(--txt-3)">
          <rect x="3" y="3" width="18" height="18" rx="3" ry="3"/>
          <line x1="8" y1="12" x2="16" y2="12"/>
          <line x1="12" y1="8" x2="12" y2="16"/>
        </svg>
        <span class="running-app-name">${escapeHtml(appName)}</span>
      </div>
      <button type="button" class="btn-add-running-app">${tr("+ Добавить")}</button>
    `;

    const btnAdd = row.querySelector('.btn-add-running-app');
    btnAdd.addEventListener('click', () => {
      addAppToList(activeAppPickerTarget, appName);
      btnAdd.textContent = tr("Добавлено");
      btnAdd.style.background = 'var(--bg-elevated)';
      btnAdd.style.color = 'var(--txt-3)';
      btnAdd.disabled = true;
    });

    runningAppsContainer.appendChild(row);
  });
}

if (inputSearchRunning) {
  inputSearchRunning.addEventListener('input', () => {
    renderRunningAppsList(cachedRunningApps);
  });
}

// Live Logs Polling & Optimization
let cachedLogsRaw = '';
let logsNeedRender = false;

function renderLogsDOM(rawLogs) {
  if (!logOutput) return;
  if (!rawLogs || rawLogs.trim() === '') {
    logOutput.innerHTML = `<div class="log-line" style="color:var(--txt-3)">${tr("[GUI] Ожидание подключения к серверу...")}</div>`;
    return;
  }
  const atBottom = (logOutput.scrollHeight - logOutput.scrollTop) <= (logOutput.clientHeight + 60);
  logOutput.innerHTML = formatLogOutput(rawLogs);
  if (atBottom) {
    logOutput.scrollTop = logOutput.scrollHeight;
  }
}

function formatLogOutput(rawLogs) {
  if (!rawLogs) return '';
  const lines = rawLogs.split('\n');
  const len = lines.length;
  // Render up to 350 most recent lines for instant 60fps responsiveness
  const start = len > 350 ? len - 350 : 0;
  const out = [];

  for (let i = start; i < len; i++) {
    const line = lines[i];
    if (!line) continue;
    let escaped = escapeHtml(line);
    // highlight timestamps
    escaped = escaped.replace(/^(\d{4}\/\d{2}\/\d{2}\s\d{2}:\d{2}:\d{2})/, '<span class="log-ts">$1</span>');
    // fast single-pass combined regex replacement
    escaped = escaped.replace(/(\[(?:REALITY|TUN|QUIC-UDP|UDP-TCP|QUIC-REALITY|QUIC-DEMUX|SOCKS5|NET|CONFIG|GUI)\]|\b(?:failed|error|CRYPTO_ERROR|mismatch|rejected|completed|success|established|authorized|active)\b)/gi, (m) => {
      const u = m.toUpperCase();
      if (u === '[REALITY]') return '<span class="log-tag-reality">' + m + '</span>';
      if (u === '[TUN]' || u.startsWith('[QUIC') || u === '[UDP-TCP]') return '<span class="log-tag-tun">' + m + '</span>';
      if (u === '[SOCKS5]') return '<span class="log-tag-socks">' + m + '</span>';
      if (u === '[NET]') return '<span class="log-tag-net">' + m + '</span>';
      if (u === '[CONFIG]' || u === '[GUI]') return '<span class="log-tag-config">' + m + '</span>';
      if (/failed|error|CRYPTO_ERROR|mismatch|rejected/i.test(m)) return '<span class="log-tag-error">' + m + '</span>';
      return '<span class="log-tag-success">' + m + '</span>';
    });
    out.push(`<div class="log-line">${escaped}</div>`);
  }
  return out.join('');
}

async function pollStatusAndLogs() {
  try {
    const res = await fetch('/api/status');
    const data = await res.json();
    if (data.connected !== isConnected) {
      setConnectedState(data.connected);
    }
    if (typeof data.logs === 'string') {
      if (data.logs !== cachedLogsRaw) {
        cachedLogsRaw = data.logs;
        if (currentActiveTab === 'tab-logs') {
          renderLogsDOM(cachedLogsRaw);
          logsNeedRender = false;
        } else {
          logsNeedRender = true;
        }
      }
    }
  } catch (e) {
    if (currentActiveTab === 'tab-logs' && logOutput && (!logOutput.textContent || logOutput.textContent.trim() === '')) {
      logOutput.innerHTML = `<div class="log-line" style="color:var(--txt-3)">${tr("[GUI] Служба ядра недоступна...")}</div>`;
    }
  }
}

// Clear / Copy Logs
const btnClearLogs = document.getElementById('btn-clear-logs');
if (btnClearLogs) {
  btnClearLogs.addEventListener('click', async () => {
    cachedLogsRaw = '';
    logsNeedRender = false;
    if (logOutput) logOutput.innerHTML = '';
    await fetch('/api/logs/clear', { method: 'POST' });
  });
}
const btnCopyLogs = document.getElementById('btn-copy-logs');
if (btnCopyLogs) {
  btnCopyLogs.addEventListener('click', () => {
    if (logOutput) {
      navigator.clipboard.writeText(logOutput.innerText || logOutput.textContent);
      showToast(tr("Журнал скопирован в буфер обмена"));
    }
  });
}

// Профили группируются по подписке; ручные подключения выводятся отдельно.

const collapsedSubscriptionGroups = new Set();
function renderGroupedProviders(servers, activeId, subscriptions) {
  const container = document.getElementById('providers-container');
  if (!container) return;
  container.innerHTML = '';

  servers = servers || [];
  const groups = Array.isArray(subscriptions) ? subscriptions : (subscriptions && subscriptions.url ? [subscriptions] : []);
  if (servers.length === 0 && groups.length === 0) {
    container.innerHTML = `<div style="color:var(--text-muted); font-size:13.5px; padding:24px 8px; text-align:center;">${tr("Нет добавленных серверов. Нажмите \"+ Добавить\" или \"Вставить\" ссылку/подписку.")}</div>`;
    return;
  }

  const manualServers = servers.filter(srv => !srv.is_subscription);
  for (const subscription of groups) {
    const subServers = servers.filter(srv => srv.subscription_id === subscription.id || (!srv.subscription_id && srv.is_subscription && groups.length === 1));
    const subSection = document.createElement('div');
    subSection.className = 'provider-section';

    const subTitle = document.createElement('div');
    subTitle.className = 'provider-category-title';
    subTitle.textContent = tr("ПОДПИСКА");
    subSection.appendChild(subTitle);

    const subBox = document.createElement('div');
    subBox.className = 'provider-box';

    const subHeader = document.createElement('div');
    subHeader.className = 'provider-box-header';

    const headerLeft = document.createElement('div');
    headerLeft.className = 'provider-header-left';
    headerLeft.innerHTML = `
      <span class="provider-chevron">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <polyline points="6 9 12 15 18 9"/>
        </svg>
      </span>
      <div class="provider-name-wrap">
        <span class="provider-name">${escapeHtml(subscription.name || 'Подписка')}</span>
        <span class="provider-subtext">${tr("{0} сервер(ов)", subServers.length)}</span>
      </div>
    `;

    const headerRight = document.createElement('div');
    headerRight.className = 'provider-header-right';
    const dateStr = subscription.last_updated || tr("Не обновлялась");
    headerRight.innerHTML = `
      <span class="provider-date">${escapeHtml(dateStr)}</span>
      <span class="provider-badge-count">${subServers.length}</span>
      <button type="button" class="provider-action-btn btn-edit-sub" title="${tr("Редактировать подписку")}">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
          <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
        </svg>
      </button>
      <button type="button" class="provider-action-btn btn-refresh-sub" title="${tr("Обновить подписку")}">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="23 4 23 10 17 10"/>
          <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
        </svg>
      </button>
      <button type="button" class="provider-action-btn btn-more-sub" title="${tr("Управление подпиской")}">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="1.5"/><circle cx="19" cy="12" r="1.5"/><circle cx="5" cy="12" r="1.5"/>
        </svg>
      </button>
    `;

    const shareSubscription = document.createElement('button');
    shareSubscription.type = 'button'; shareSubscription.className = 'provider-action-btn';
    shareSubscription.textContent = 'QR'; shareSubscription.title = tr("Ссылка и QR-код подписки");
    shareSubscription.setAttribute('aria-label', shareSubscription.title);
    shareSubscription.disabled = !subscription.url;
    shareSubscription.addEventListener('click', event => {
      event.stopPropagation();
      AtlantShare.open(subscription.url, subscription.name || tr("Подписка"), {qr:tr("QR-код подписки"),link:tr("Ссылка подписки"),copy:tr("Копировать"),copied:tr("Скопировано"),close:tr("Закрыть"),warning:tr("Ссылка даёт доступ к VPN. Передавайте её только тем, кому доверяете."),tooLong:tr("Не удалось создать QR-код. Скопируйте ссылку.")});
    });
    headerRight.prepend(shareSubscription);

    subHeader.appendChild(headerLeft);
    subHeader.appendChild(headerRight);
    subBox.appendChild(subHeader);

    // Traffic line (ONLY for subscriptions!)
    const trafficRow = document.createElement('div');
    trafficRow.className = 'provider-traffic-row';
    const usage = AtlantUsage.summary(subscription);
    const remainText = usage.text;
    trafficRow.innerHTML = `
      <span class="traffic-left-symbol">${tr("Трафик")}</span>
      <span class="traffic-right-val">${escapeHtml(remainText)}</span>
    `;
    subBox.appendChild(trafficRow);
    if (usage.finite) {
      const meter = document.createElement('progress');
      meter.className = 'subscription-quota';
      meter.max = 100; meter.value = usage.percent;
      meter.setAttribute('aria-label', tr("Использовано ") + usage.percent.toFixed(1) + tr("% лимита"));
      subBox.appendChild(meter);
    }

    if (subscription.expire > 0) {
      const expiryRow = document.createElement('div');
      expiryRow.className = 'provider-traffic-row';
      expiryRow.textContent = tr("Действует до ") + new Date(subscription.expire * 1000).toLocaleDateString();
      subBox.appendChild(expiryRow);
    }
    if (subscription.announcement) {
      const announcementRow = document.createElement('div');
      announcementRow.className = 'provider-traffic-row';
      announcementRow.textContent = subscription.announcement;
      subBox.appendChild(announcementRow);
    }
    if (subscription.usage_scope === 'local-account' || subscription.usage_scope === 'global-subscription') {
      const scopeRow = document.createElement('div');
      scopeRow.className = 'provider-traffic-row';
      scopeRow.textContent = subscription.usage_scope === 'global-subscription' ? tr("Общий трафик подписки на всех подключённых VDS") : tr("Трафик учётной записи на сервере подписки");
      subBox.appendChild(scopeRow);
    }

    const subServersList = document.createElement('div');
    subServersList.className = 'provider-servers-list';
    const groupKey = subscription.id || subscription.url;
    if (collapsedSubscriptionGroups.has(groupKey)) {
      subServersList.classList.add('collapsed');
      headerLeft.querySelector('.provider-chevron').classList.add('collapsed');
    }

    headerLeft.addEventListener('click', () => {
      const chevron = headerLeft.querySelector('.provider-chevron');
      if (subServersList.classList.contains('collapsed')) {
        subServersList.classList.remove('collapsed');
        collapsedSubscriptionGroups.delete(groupKey);
        chevron.classList.remove('collapsed');
      } else {
        subServersList.classList.add('collapsed');
        collapsedSubscriptionGroups.add(groupKey);
        chevron.classList.add('collapsed');
      }
    });

    const btnEditSub = headerRight.querySelector('.btn-edit-sub');
    if (btnEditSub) {
      btnEditSub.addEventListener('click', (e) => {
        e.stopPropagation();
        openEditSubscriptionModal(subscription);
      });
    }

    const btnMore = headerRight.querySelector('.btn-more-sub');
    if (btnMore) {
      btnMore.addEventListener('click', async (e) => {
        e.stopPropagation();
        if (confirm(tr("Удалить подписку \"{0}\" и её сервера?", subscription.name || 'Подписка'))) {
          try {
            const res = await fetch('/api/subscription/clear', { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({id: subscription.id}) });
            if (res.ok) {
              showToast(tr("Подписка удалена"));
              await loadConfig();
            }
          } catch (err) {
            showToast(tr("Ошибка: ") + err.message);
          }
        }
      });
    }

    const btnRefresh = headerRight.querySelector('.btn-refresh-sub');
    if (btnRefresh) {
      btnRefresh.addEventListener('click', async (e) => {
        e.stopPropagation();
        if (subscription.url) {
          showToast(tr("Обновление подписки..."));
          try {
            const res = await fetch('/api/subscription/update', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ id: subscription.id, url: subscription.url })
            });
            const d = await res.json();
            if (d.success) {
              showToast(tr("Подписка обновлена! Серверов: {0}", d.count));
              await loadConfig();
            } else {
              showToast(tr("Ошибка: ") + (d.error || tr("Сбой обновления")));
            }
          } catch (err) {
            showToast(tr("Сбой обновления: ") + err.message);
          }
        } else {
          showToast(tr("URL подписки не указан"));
        }
      });
    }

    subServers.forEach(srv => {
      subServersList.appendChild(createServerCardElement(srv, activeId));
    });

    subBox.appendChild(subServersList);
    subSection.appendChild(subBox);
    container.appendChild(subSection);
  }

  // 3. Render Standalone/Custom Servers (Clean list without subscription traffic header)
  if (manualServers.length > 0) {
    const srvSection = document.createElement('div');
    srvSection.className = 'provider-section';

    const srvTitle = document.createElement('div');
    srvTitle.className = 'provider-category-title';
    srvTitle.textContent = tr("СЕРВЕРА");
    srvSection.appendChild(srvTitle);

    const srvBox = document.createElement('div');
    srvBox.className = 'provider-box';

    const srvListEl = document.createElement('div');
    srvListEl.className = 'provider-servers-list';

    manualServers.forEach(srv => {
      srvListEl.appendChild(createServerCardElement(srv, activeId));
    });

    srvBox.appendChild(srvListEl);
    srvSection.appendChild(srvBox);
    container.appendChild(srvSection);
  }
}

function createServerCardElement(srv, activeId) {
  const srvCard = document.createElement('div');
  const isActive = (srv.id === activeId);
  srvCard.className = 'prov-server-card' + (isActive ? ' active' : '');
  srvCard.dataset.id = srv.id;

  const proto = (srv.type || 'atlant').toUpperCase();
  const transport = (srv.transport || 'quic').toUpperCase();
  const mode = srv.traffic_mode ? `· ${srv.traffic_mode.toUpperCase()}` : '';
  const tagsText = `${transport} ${mode}`.trim();
  const numericPing = Number(srv.ping);
  const pingVal = Number.isFinite(numericPing) && numericPing > 0 ? `<span class="prov-ping-badge ${numericPing < 120 ? 'good' : 'warn'}">${Math.round(numericPing)} ms</span>` : '<span class="prov-ping-badge" style="display:none;"></span>';

  srvCard.innerHTML = `
    <div class="prov-server-left">
      <div class="prov-server-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
          <circle cx="12" cy="12" r="10"/>
          <line x1="2" y1="12" x2="22" y2="12"/>
          <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>
        </svg>
      </div>
      <div class="prov-server-info">
        <div class="prov-server-name">${escapeHtml(srv.name || srv.server)}</div>
        <div class="prov-server-tags">
          <span class="tag-protocol">${escapeHtml(proto)}</span>
          ${tagsText ? `<span class="tag-details">· ${escapeHtml(tagsText)}</span>` : ''}
          ${srv.group ? `<span class="tag-group" style="font-size:10.5px; opacity:0.75; margin-left:4px;">[${escapeHtml(srv.group)}]</span>` : ''}
        </div>
      </div>
    </div>
    <div class="prov-server-right">
      ${pingVal}
      <button type="button" class="btn-server-edit" title="${tr("Редактировать параметры")}" data-id="${escapeHtml(srv.id)}">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="15" height="15">
          <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
          <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
        </svg>
      </button>
      <button type="button" class="btn-server-more" title="${tr("Параметры сервера")}" data-id="${escapeHtml(srv.id)}">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
          <circle cx="12" cy="12" r="1.5"/><circle cx="19" cy="12" r="1.5"/><circle cx="5" cy="12" r="1.5"/>
        </svg>
      </button>
    </div>
  `;

  srvCard.addEventListener('click', async (e) => {
    if (e.target.closest('.btn-server-more') || e.target.closest('.btn-server-edit')) return;
    await selectServer(srv.id);
  });

  const btnEdit = srvCard.querySelector('.btn-server-edit');
  if (btnEdit) {
    btnEdit.addEventListener('click', (e) => {
      e.stopPropagation();
      openEditConfigModal(srv);
    });
  }

  const btnMore = srvCard.querySelector('.btn-server-more');
  if (btnMore) {
    btnMore.addEventListener('click', (e) => {
      e.stopPropagation();
      openEditConfigModal(srv);
    });
  }

  return srvCard;
}

function escapeHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

async function selectServer(id) {
  try {
    const res = await fetch('/api/servers/select', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id })
    });
    if (res.ok) {
      showToast(tr("Сервер переключен"));
      await loadConfig();
      // если уже подключены - перезапускаем туннель на новый сервер
      if (isConnected) {
        showToast(tr("Переподключение к выбранному серверу..."));
        await stopVPN();
        setTimeout(startVPN, 500);
      }
    }
  } catch (e) {
    showToast(tr("Сбой выбора сервера: ") + e.message);
  }
}

async function deleteServer(id) {
  try {
    const res = await fetch('/api/servers/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id })
    });
    if (res.ok) {
      showToast(tr("Сервер успешно удален"));
      await loadConfig();
    } else {
      showToast(tr("Ошибка удаления сервера"));
    }
  } catch (e) {
    showToast(tr("Сбой удаления: ") + e.message);
  }
}

// Редактор профиля подключения.
const modalEditConfig = document.getElementById('modal-edit-config');
const editServerJson = document.getElementById('edit-server-json');
const btnCloseEditModal = document.getElementById('btn-close-edit-modal');
const btnCancelEdit = document.getElementById('btn-cancel-edit');
const btnSaveEditServer = document.getElementById('btn-save-edit-server');
const btnDeleteServer = document.getElementById('btn-delete-server');
const btnTestPingModal = document.getElementById('btn-test-ping-modal');
const btnShareServer = document.getElementById('btn-share-server');

// Modal Tabs: Parameters vs JSON
const tabBtnFields = document.getElementById('tab-btn-fields');
const tabBtnJson = document.getElementById('tab-btn-json');
const editPaneFields = document.getElementById('edit-pane-fields');
const editPaneJson = document.getElementById('edit-pane-json');

// Form inputs
const edSrvName = document.getElementById('ed-srv-name');
const edSrvHost = document.getElementById('ed-srv-host');
const edSrvSni = document.getElementById('ed-srv-sni');
const edSrvUser = document.getElementById('ed-srv-user');
const edSrvPubkey = document.getElementById('ed-srv-pubkey');
const edSrvPbk = document.getElementById('ed-srv-pbk');
const edSrvGroup = document.getElementById('ed-srv-group');
const edSrvSecurity = document.getElementById('ed-srv-security');

if (tabBtnFields && tabBtnJson) {
  tabBtnFields.addEventListener('click', () => {
    tabBtnFields.classList.add('active');
    tabBtnJson.classList.remove('active');
    editPaneFields.classList.add('active');
    editPaneJson.classList.remove('active');
  });

  tabBtnJson.addEventListener('click', () => {
    syncFormToJson();
    tabBtnJson.classList.add('active');
    tabBtnFields.classList.remove('active');
    editPaneJson.classList.add('active');
    editPaneFields.classList.remove('active');
  });
}

function profileWireVersion(profile) {
  const version = profile && profile.wire_version != null ? profile.wire_version : (profile ? 4 : 6);
  if (![2, 3, 4, 6, 7].includes(version)) throw new Error(tr("Поддерживаются форматы 2.x, 3.x, 4.x, 6.x и 7.x"));
  return version;
}

function syncFormToJson() {
  const isReality = (edSrvSecurity ? edSrvSecurity.value : 'reality') === 'reality';
  const existing = (globalConfig && globalConfig.servers ? globalConfig.servers.find(s => s.id === editingServerId) : null);
  const effTransport = (existing && existing.transport) || (globalConfig && globalConfig.transport) || 'reality';
  const effMode = (existing && existing.traffic_mode) || (globalConfig && globalConfig.traffic_mode) || 'split';

  // чистый конфиг для Atlant ядра
  const current = {
    name: edSrvName ? edSrvName.value : "",
    server: edSrvHost ? edSrvHost.value : "",
    sni: edSrvSni ? edSrvSni.value : "www.microsoft.com",
    server_pub_key: edSrvPubkey ? edSrvPubkey.value : "",
    pbk: edSrvPbk ? edSrvPbk.value : "",
    user: edSrvUser ? edSrvUser.value : "test",
    group: edSrvGroup ? edSrvGroup.value : (existing ? existing.group : ""),
    transport: effTransport,
    wire_version: profileWireVersion(existing),
    traffic_mode: effMode,
    security: isReality ? 'reality' : 'tls',
    type: "atlant"
  };
  if (editServerJson) {
    editServerJson.value = JSON.stringify(current, null, 2);
  }
}

function openEditConfigModal(srv) {
  editingServerId = srv.id;

  const titleEl = document.getElementById('edit-modal-title');
  if (titleEl) titleEl.textContent = tr("Настройки: {0}", srv.name || srv.server);

  // Populate visual inputs
  if (edSrvName) edSrvName.value = srv.name || '';
  if (edSrvHost) edSrvHost.value = srv.server || '';
  if (edSrvSni) edSrvSni.value = srv.sni || 'www.microsoft.com';
  if (edSrvUser) edSrvUser.value = srv.user || 'test';
  if (edSrvPubkey) edSrvPubkey.value = srv.server_pub_key || '';
  if (edSrvPbk) edSrvPbk.value = srv.pbk || '';
  if (edSrvGroup) edSrvGroup.value = srv.group || '';

  if (edSrvSecurity) {
    edSrvSecurity.value = (srv.security === 'tls') ? 'tls' : 'reality';
  }

  // Build JSON
  syncFormToJson();

  // Reset to Parameters tab
  if (tabBtnFields) tabBtnFields.click();

  const pingLbl = document.getElementById('modal-ping-label');
  if (pingLbl) pingLbl.textContent = tr("Пинг");

  if (modalEditConfig) modalEditConfig.classList.add('show');
}

function closeEditModal() {
  if (modalEditConfig) modalEditConfig.classList.remove('show');
  editingServerId = null;
}

if (btnCloseEditModal) btnCloseEditModal.addEventListener('click', closeEditModal);
if (btnCancelEdit) btnCancelEdit.addEventListener('click', closeEditModal);

// Action: Delete Server
if (btnDeleteServer) {
  btnDeleteServer.addEventListener('click', async () => {
    if (!editingServerId) return;
    if (confirm(tr("Вы уверены, что хотите удалить этот сервер?"))) {
      const idToDelete = editingServerId;
      closeEditModal();
      await deleteServer(idToDelete);
    }
  });
}

// Action: Test Ping from Modal
if (btnTestPingModal) {
  btnTestPingModal.addEventListener('click', async () => {
    const srvHost = edSrvHost ? edSrvHost.value.trim() : '';
    const pingLbl = document.getElementById('modal-ping-label');
    if (pingLbl) pingLbl.textContent = '...';
    const rtt = await checkPing(srvHost);
    if (pingLbl) {
      pingLbl.textContent = rtt >= 0 ? `${rtt} ms` : tr("Ошибка");
    }
  });
}

// Action: Share / Copy Link
if (btnShareServer) {
  btnShareServer.addEventListener('click', () => {
    const srv = edSrvHost ? edSrvHost.value.trim() : '';
    const user = edSrvUser ? edSrvUser.value.trim() : 'test';
    const sni = edSrvSni ? edSrvSni.value.trim() : 'www.microsoft.com';
    const pbk = edSrvPbk ? edSrvPbk.value.trim() : '';
    const spk = edSrvPubkey ? edSrvPubkey.value.trim() : '';
    const name = edSrvName ? edSrvName.value.trim() : '';
    const existing = (globalConfig && globalConfig.servers ? globalConfig.servers.find(s => s.id === editingServerId) : null);
    const fp = (existing && existing.fingerprint) || (globalConfig && globalConfig.fingerprint) || 'firefox';
    const transport = (existing && existing.transport) || (globalConfig && globalConfig.transport) || 'reality';
    const mode = (existing && existing.traffic_mode) || (globalConfig && globalConfig.traffic_mode) || 'split';
    const wireVersion = profileWireVersion(existing);
    const link = `atlant://${encodeURIComponent(user)}@${srv}?sni=${encodeURIComponent(sni)}&user=${encodeURIComponent(user)}&pbk=${encodeURIComponent(pbk)}&server_pub_key=${encodeURIComponent(spk)}&transport=${encodeURIComponent(transport)}&traffic_mode=${encodeURIComponent(mode)}&wire_version=${wireVersion}&fp=${encodeURIComponent(fp)}#${encodeURIComponent(name || srv)}`;
    navigator.clipboard.writeText(link);
    showToast(tr("Ссылка atlant:// скопирована в буфер обмена"));
  });
}

// Save edited server
if (btnSaveEditServer) {
  btnSaveEditServer.addEventListener('click', async () => {
    if (!editingServerId) return;

    let srvName = edSrvName ? edSrvName.value.trim() : '';
    let srvHost = edSrvHost ? edSrvHost.value.trim() : '';
    let srvSni = edSrvSni ? edSrvSni.value.trim() : 'www.microsoft.com';
    let srvUser = edSrvUser ? edSrvUser.value.trim() : 'test';
    let srvSpk = edSrvPubkey ? edSrvPubkey.value.trim() : '';
    let srvPbk = edSrvPbk ? edSrvPbk.value.trim() : '';
    let srvSecurity = edSrvSecurity ? edSrvSecurity.value : 'reality';
    let raw = editServerJson ? editServerJson.value.trim() : '';

    const existing = (globalConfig && globalConfig.servers ? globalConfig.servers.find(s => s.id === editingServerId) : null);
    let effTransport = (existing && existing.transport) || (globalConfig && globalConfig.transport) || 'reality';
    let effMode = (existing && existing.traffic_mode) || (globalConfig && globalConfig.traffic_mode) || 'split';
    let wireVersion = profileWireVersion(existing);
    let srvGroup = edSrvGroup ? edSrvGroup.value.trim() : (existing ? existing.group : '');

    // If currently on JSON tab, parse from JSON
    if (editPaneJson && editPaneJson.classList.contains('active')) {
      try {
        const parsed = JSON.parse(raw);
        if (parsed.server) srvHost = parsed.server;
        if (parsed.name) srvName = parsed.name;
        if (parsed.sni) srvSni = parsed.sni;
        if (parsed.pbk) srvPbk = parsed.pbk;
        if (parsed.server_pub_key) srvSpk = parsed.server_pub_key;
        if (parsed.user) srvUser = parsed.user;
        if (parsed.group !== undefined) srvGroup = parsed.group;
        if (parsed.transport) effTransport = parsed.transport;
        if (parsed.traffic_mode) effMode = parsed.traffic_mode;
        if (parsed.security) srvSecurity = parsed.security;
        if (parsed.wire_version !== undefined) wireVersion = profileWireVersion(parsed);
      } catch (e) {
        showToast(tr("Некорректный JSON синтаксис: ") + e.message);
        return;
      }
    }

    if (!srvHost) {
      showToast(tr("Адрес сервера не может быть пустым"));
      return;
    }

    const payload = {
      id: editingServerId,
      name: srvName || srvHost,
      server: srvHost,
      sni: srvSni,
      server_pub_key: srvSpk,
      pbk: srvPbk,
      user: srvUser,
      group: srvGroup,
      transport: effTransport,
      wire_version: wireVersion,
      security: srvSecurity,
      traffic_mode: effMode,
      type: 'atlant',
      raw_json: raw
    };

    try {
      const res = await fetch('/api/servers/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      if (res.ok) {
        closeEditModal();
        showToast(tr("Сервер сохранен"));
        await loadConfig();
      } else {
        const err = await res.text();
        showToast(tr("Ошибка сохранения: ") + err);
      }
    } catch (e) {
      showToast(tr("Сбой сохранения: ") + e.message);
    }
  });
}

// Редактор подписки.
const modalEditSub = document.getElementById('modal-edit-subscription');
const subEditName = document.getElementById('sub-edit-name');
const subEditUrl = document.getElementById('sub-edit-url');
const btnCloseSubModal = document.getElementById('btn-close-sub-modal');
const btnCancelSubModal = document.getElementById('btn-cancel-sub-modal');
const btnSaveSubModal = document.getElementById('btn-save-sub-modal');
const btnDeleteSubModal = document.getElementById('btn-delete-sub-modal');
let editingSubscriptionId = '';

function openEditSubscriptionModal(sub) {
  editingSubscriptionId = sub && sub.id || '';
  if (subEditName) subEditName.value = (sub && sub.name) || (globalConfig && globalConfig.subscription ? globalConfig.subscription.name : '') || '';
  if (subEditUrl) subEditUrl.value = (sub && sub.url) || (globalConfig && globalConfig.subscription ? globalConfig.subscription.url : '') || '';
  if (modalEditSub) modalEditSub.classList.add('show');
}

function closeEditSubscriptionModal() {
  if (modalEditSub) modalEditSub.classList.remove('show');
}

if (btnCloseSubModal) btnCloseSubModal.addEventListener('click', closeEditSubscriptionModal);
if (btnCancelSubModal) btnCancelSubModal.addEventListener('click', closeEditSubscriptionModal);

if (btnDeleteSubModal) {
  btnDeleteSubModal.addEventListener('click', async () => {
    if (confirm(tr("Удалить эту подписку и её серверы?"))) {
      closeEditSubscriptionModal();
      try {
        const res = await fetch('/api/subscription/clear', { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({id: editingSubscriptionId}) });
        if (res.ok) {
          showToast(tr("Подписка удалена"));
          await loadConfig();
        }
      } catch (e) {
        showToast(tr("Ошибка удаления: ") + e.message);
      }
    }
  });
}

if (btnSaveSubModal) {
  btnSaveSubModal.addEventListener('click', async () => {
    const name = subEditName ? subEditName.value.trim() : '';
    const url = subEditUrl ? subEditUrl.value.trim() : '';
    try {
      const res = await fetch('/api/subscription/edit', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: editingSubscriptionId, name, url })
      });
      if (res.ok) {
        closeEditSubscriptionModal();
        showToast(tr("Подписка успешно сохранена"));
        await loadConfig();
      } else {
        const err = await res.text();
        showToast(tr("Ошибка сохранения подписки: ") + err);
      }
    } catch (e) {
      showToast(tr("Сбой сохранения: ") + e.message);
    }
  });
}

// Действия списка подключений.
const btnPasteClipboard = document.getElementById('btn-paste-clipboard');
if (btnPasteClipboard) {
  btnPasteClipboard.addEventListener('click', async () => {
    if (btnPasteClipboard.disabled) return;
    btnPasteClipboard.disabled = true;
    try {
      let text;
      try {
        text = await navigator.clipboard.readText();
      } catch (_) {
        // App-mode browsers may deny Clipboard API access. The authenticated
        // local backend can read Windows Unicode text without browser permission.
        const clipboardResponse = await fetch('/api/clipboard/text', { method: 'POST' });
        if (!clipboardResponse.ok) throw new Error(await clipboardResponse.text());
        text = (await clipboardResponse.json()).text;
      }
      if (!text || !text.trim()) {
        showToast(tr("Буфер обмена пуст"));
        return;
      }
      showToast(tr("Импорт из буфера обмена..."));
      const res = await fetch('/api/servers/paste', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: text.trim() })
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      if (data.success) {
        showToast(tr("Импортировано: {0} сервер(ов)", data.added_count ?? 0));
        await loadConfig();
      } else {
        showToast(tr("Ошибка импорта: ") + (data.error || tr("Неверная ссылка")));
      }
    } catch (err) {
      showToast(tr("Ошибка импорта: ") + err.message);
    } finally {
      btnPasteClipboard.disabled = false;
    }
  });
}

// Ping All Servers
const btnPingAll = document.getElementById('btn-ping-all');
if (btnPingAll) {
  btnPingAll.addEventListener('click', async () => {
    showToast(tr("Проверка пинга..."));
    if (globalConfig && globalConfig.servers) {
      for (const s of globalConfig.servers) {
        const rtt = await checkPing(s.server);
        s.ping = rtt;
        const card = document.querySelector(`.prov-server-card[data-id="${s.id}"]`);
        if (card) {
          const badge = card.querySelector('.prov-ping-badge');
          if (badge) {
            badge.style.display = 'inline-block';
            badge.textContent = rtt >= 0 ? `${rtt} ms` : tr("Ошибка");
            badge.className = `prov-ping-badge ${rtt >= 0 && rtt < 120 ? 'good' : 'warn'}`;
          }
        }
      }
      showToast(tr("Проверка пинга завершена"));
    }
  });
}

// Add Server Modal
const modalAddServer = document.getElementById('modal-add-server');
const btnAddServerModal = document.getElementById('btn-add-server-modal');
const btnCloseServerModal = document.getElementById('btn-close-server-modal');
const btnCancelAddServer = document.getElementById('btn-cancel-add-server');
const btnSaveNewServer = document.getElementById('btn-save-new-server');

if (btnAddServerModal && modalAddServer) {
  btnAddServerModal.addEventListener('click', () => {
    modalAddServer.classList.add('show');
  });
}

function closeAddModal() {
  if (modalAddServer) modalAddServer.classList.remove('show');
}
if (btnCloseServerModal) btnCloseServerModal.addEventListener('click', closeAddModal);
if (btnCancelAddServer) btnCancelAddServer.addEventListener('click', closeAddModal);

if (btnSaveNewServer) {
  btnSaveNewServer.addEventListener('click', async () => {
    const name = document.getElementById('m-srv-name')?.value.trim();
    const server = document.getElementById('m-srv-host')?.value.trim();
    const sni = document.getElementById('m-srv-sni')?.value.trim() || 'www.microsoft.com';
    const spk = document.getElementById('m-srv-pubkey')?.value.trim() || '';
    const pbk = document.getElementById('m-srv-pbk')?.value.trim() || '';
    const user = document.getElementById('m-srv-user')?.value.trim() || 'test';

    if (!server) {
      showToast(tr("Укажите адрес сервера (IP:Port)"));
      return;
    }

    const transport = (globalConfig && globalConfig.transport) || 'reality';
    const mode = (globalConfig && globalConfig.traffic_mode) || 'split';

    const payload = {
      name: name || server,
      server: server,
      sni: sni,
      server_pub_key: spk,
      pbk: pbk,
      user: user,
      transport: transport,
      traffic_mode: mode,
      type: 'atlant'
    };

    try {
      const res = await fetch('/api/servers/add', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      if (res.ok) {
        closeAddModal();
        showToast(tr("Сервер добавлен"));
        await loadConfig();
      }
    } catch (e) {
      showToast(tr("Ошибка добавления сервера: ") + e.message);
    }
  });
}

// закрытие окон по esc
window.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    closeAddModal();
    closeEditModal();
  }
});

const btnExitApp = document.getElementById('btn-exit-app');
if (btnExitApp) {
  btnExitApp.addEventListener('click', async () => {
    if (confirm(tr("Завершить работу Atlant VPN?"))) {
      try {
        await fetch('/api/shutdown', { method: 'POST' });
      } catch (e) {}
      window.close();
    }
  });
}

// стартовая инициализация и опрос статуса
// Closing the window releases the connection. Reloads reconnect within grace;
// minimized windows do not depend on JavaScript heartbeat timer frequency.
const windowLifetime = new EventSource('/api/ui/lifetime');
window.addEventListener('pagehide', () => windowLifetime.close());
window.addEventListener('pageshow', event => {
  if (event.persisted) window.location.reload();
});
loadConfig();
checkPing();
setInterval(pollStatusAndLogs, 1000);
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) {
    pollStatusAndLogs();
  }
});
