const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const {format} = require('../../client/windows/gui/web/i18n.js');
const ruText = (source, ...values) => format(source, values);
const source = fs.readFileSync(path.join(__dirname, '../../client/windows/gui/web/app.js'), 'utf8');
const routing = source.slice(source.indexOf('let configWriteQueue ='), source.indexOf('// TUN & KillSwitch toggle listeners'));
const settings = source.slice(source.indexOf('async function saveGlobalSettings('), source.indexOf('const btnSettingsResetNet'));

function boot({fail=false, mtu='1360', loaded=true}={}) {
  const elements = {
    'set-mtu': {value:mtu}, 'set-dns': {value:'1.1.1.1'},
    'set-traffic-mode': {value:'split'}, 'set-tcp-proto': {value:'xhttp'},
    'set-udp-proto': {value:'tcp'}, 'set-min-padding': {value:'0'},
  };
  const requests=[], notices=[];
  const context = {
    tr: ruText,
    globalConfig: loaded ? {server:'example.invalid:443', marker:'preserved', chaffing_min_bytes:23, chaffing_max_bytes:99} : null,
    currentRouteMode:'bypass', proxyAppsList:['browser.exe'], bypassAppsList:['other.exe'],
    document:{getElementById(id){return elements[id] || null}},
    checkTun:null, checkKillswitch:null, isConnected:false,
    showToast(message){notices.push(message)},
    async fetch(url, options) {
      if (!options) throw Error('unavailable');
      requests.push(JSON.parse(options.body));
      return {ok:!fail};
    }, setTimeout(){},
  };
  vm.createContext(context);
  vm.runInContext(routing+'\n'+settings,context);
  return {context,requests,notices,elements};
}
test('settings save has no undefined silent reference and preserves XHTTP / zero padding',async()=>{
  const ui=boot();
  assert.equal(await ui.context.saveGlobalSettings(),true);
  assert.equal(ui.requests[0].tcp_proto,'xhttp');
  assert.equal(ui.requests[0].transport,'xhttp');
  assert.equal(ui.requests[0].min_padding,0);
  assert.equal(ui.requests[0].marker,'preserved');
  assert.equal(ui.requests[0].chaffing_min_bytes,23);
  assert.equal(ui.requests[0].chaffing_max_bytes,99);
  assert.ok(ui.notices.includes('Настройки сохранены'));
});
test('invalid and incomplete MTU is not replaced by a default',async()=>{
  for(const mtu of ['', '13', '1501', 'bad']) {
    const ui=boot({mtu});
    assert.equal(await ui.context.saveGlobalSettings(),false);
    assert.equal(ui.requests.length,0);
  }
});
test('save failure is reported and does not replace committed config',async()=>{
  const ui=boot({fail:true});
  assert.equal(await ui.context.saveGlobalSettings(),false);
  assert.equal(ui.context.globalConfig.transport,undefined);
  assert.match(ui.notices[0],/Сбой сохранения/);
});
test('failed initial load cannot overwrite config with UI defaults',async()=>{
  const ui=boot({loaded:false});
  assert.equal(await ui.context.saveGlobalSettings(),false);
  assert.equal(ui.requests.length,0);
});
test('queued routing save retains committed global settings',async()=>{
  const ui=boot();
  const results=await Promise.all([ui.context.saveGlobalSettings(true),ui.context.saveConfig(true)]);
  assert.deepEqual(results,[true,true]);
  assert.equal(ui.requests.length,2);
  assert.equal(ui.requests[1].transport,'xhttp');
  assert.equal(ui.requests[1].marker,'preserved');
  assert.equal(ui.context.globalConfig.route_mode,'bypass');
});
test('loading existing XHTTP is not mapped to reality',()=>{
  assert.match(source,/setTcpProto\.value = cfg\.tcp_proto === 'xhttp' \? 'xhttp' : 'reality'/);
});

test('TLS menu preserves imported profiles without a silent fallback',()=>{
  const select={options:[{value:'chrome'},{value:'firefox'}],value:'',
    querySelectorAll(){return this.options.filter(option=>option.dataset?.importedProfile)},
    appendChild(option){option.remove=()=>{this.options=this.options.filter(item=>item!==option)};this.options.push(option)},
  };
  const context={tr:ruText,document:{createElement(){return {dataset:{}}}}};
  vm.createContext(context);
  vm.runInContext(source.slice(source.indexOf('function selectStoredFingerprint('),source.indexOf('async function loadConfig(')),context);
  context.selectStoredFingerprint(select,'');
  assert.equal(select.value,'firefox');
  for(const value of ['android','randomized','unknown-profile']) {
    context.selectStoredFingerprint(select,value);
    assert.equal(select.value,value);
    assert.equal(select.options.length,3);
    assert.match(select.options[2].textContent,/импортирован/);
  }
  context.selectStoredFingerprint(select,'chrome');
  assert.equal(select.options.length,2);
  assert.equal(select.value,'chrome');
  for (const value of ['chrome_144', 'chrome_133', 'chrome_auto']) {
    context.selectStoredFingerprint(select, value);
    assert.equal(select.value, 'chrome');
    assert.equal(select.options.length, 2);
  }
});

test('normal TLS menu does not offer incompatible OkHttp or random browser identity',()=>{
  const html=fs.readFileSync(path.join(__dirname,'../../client/windows/gui/web/index.html'),'utf8');
  const menu=html.slice(html.indexOf('<select id="set-fingerprint"'),html.indexOf('</select>',html.indexOf('<select id="set-fingerprint"')));
  assert.doesNotMatch(menu,/value="(?:android|randomized)"/);
  assert.match(menu,/Chrome 144/);
  assert.match(menu,/value="chrome_120"/);
  assert.doesNotMatch(menu,/Chrome 133|value="chrome_133"/);
});

test('profile editor retains wire3 and rejects unknown format instead of downgrading',async()=>{
  const listeners = {}, requests = [], notices = [];
  const field = value => ({value});
  const context = {
    tr: ruText,
    globalConfig: {servers:[{id:'node',transport:'xhttp',traffic_mode:'split',wire_version:3}]},
    editingServerId:'node', edSrvName:field('Test'), edSrvHost:field('example.invalid:443'),
    edSrvSni:field('example.invalid'), edSrvPubkey:field('pin'), edSrvPbk:field('pbk'),
    edSrvUser:field('alice'), edSrvGroup:field('Test'), edSrvSecurity:field('reality'),
    editServerJson:field(''), editPaneJson:{classList:{contains:()=>false}},
    btnSaveEditServer:{addEventListener(name,callback){listeners[name]=callback}},
    closeEditModal(){}, async loadConfig(){}, showToast(message){notices.push(message)},
    async fetch(url,options){requests.push(JSON.parse(options.body)); return {ok:true}},
  };
  vm.createContext(context);
  vm.runInContext(source.slice(source.indexOf('function profileWireVersion('),source.indexOf('function openEditConfigModal(')),context);
  vm.runInContext(source.slice(source.indexOf('if (btnSaveEditServer)'),source.indexOf('if (btnSaveEditServer)')+source.slice(source.indexOf('if (btnSaveEditServer)')).indexOf('\n// ',10)),context);
  context.syncFormToJson();
  assert.equal(JSON.parse(context.editServerJson.value).wire_version,3);
  await listeners.click();
  assert.equal(requests[0].wire_version,3);
  assert.equal(context.profileWireVersion({}),4);
  assert.equal(context.profileWireVersion(null),6);
  assert.equal(context.profileWireVersion({wire_version:7}),7);
  assert.equal(context.profileWireVersion({wire_version:4}),4);
  context.globalConfig.servers[0].wire_version=6;
  context.globalConfig.servers[0].admission_tickets='opaque-fixture';
  context.syncFormToJson();
  assert.equal(JSON.parse(context.editServerJson.value).admission_tickets,undefined);
  await listeners.click();
  assert.equal(requests[1].wire_version,6);
  assert.equal(requests[1].admission_tickets,undefined);
  assert.throws(()=>context.profileWireVersion({wire_version:5}));
  assert.throws(()=>context.profileWireVersion({wire_version:999}));
  context.editPaneJson.classList.contains=()=>true;
  context.editServerJson.value='{"wire_version":999}';
  await listeners.click();
  assert.equal(requests.length,2);
  assert.ok(notices.some(value=>value.includes('2.x')));
});

test('copy profile link uses declared transport and preserves wire3',()=>{
  let click, copied;
  const context = {
    tr: ruText,
    btnShareServer:{addEventListener(name,callback){click=callback}},
    edSrvHost:{value:'[2001:db8::1]:443'},edSrvUser:{value:'alice'},
    edSrvSni:{value:'example.invalid'},edSrvPbk:{value:'secret'},edSrvPubkey:{value:'pin'},edSrvName:{value:'Node'},
    globalConfig:{servers:[{id:'node',transport:'quic',wire_version:3}]},editingServerId:'node',
    navigator:{clipboard:{writeText(value){copied=value}}},showToast(){},
  };
  vm.createContext(context);
  vm.runInContext(source.slice(source.indexOf('function profileWireVersion('),source.indexOf('function syncFormToJson(')),context);
  vm.runInContext(source.slice(source.indexOf('if (btnShareServer)'),source.indexOf('// Save edited server')),context);
  click();
  const link=new URL(copied);
  assert.equal(link.searchParams.get('wire_version'),'3');
  assert.equal(link.searchParams.get('transport'),'quic');
  assert.equal(link.searchParams.get('user'),'alice');
  assert.equal(link.hostname,'[2001:db8::1]');
  context.globalConfig.servers[0].wire_version=6;
  context.globalConfig.servers[0].admission_tickets='opaque-fixture';
  click();
  const wire6=new URL(copied);
  assert.equal(wire6.searchParams.get('wire_version'),'6');
  assert.equal(wire6.searchParams.has('admission_tickets'),false);
});
