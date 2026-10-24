const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const path=require('node:path');
const source=fs.readFileSync(path.join(__dirname,'../../client/windows/gui/web/app.js'),'utf8');
const code=source.slice(source.indexOf('async function checkPing('),source.indexOf('// Reset Network'));
function boot(fetch){
  const notices=[];
  const context={tr:source=>source,pingValue:{textContent:'',title:'',style:{}},fetch,showToast(message){notices.push(message)}};
  vm.createContext(context);vm.runInContext(code,context);
  return {context,notices};
}
test('ping distinguishes no selected server from unreachable VPN',async()=>{
  const ui=boot(async()=>({ok:false,json:async()=>({rtt:-1,error_code:'no_server',error:'Выберите сервер'})}));
  assert.equal(await ui.context.checkPing(),-1);
  assert.equal(ui.context.pingValue.textContent,'Выберите сервер');
  assert.deepEqual(ui.notices,['Выберите сервер']);
});
test('ping accepts zero RTT and identifies TCP latency',async()=>{
  const ui=boot(async()=>({ok:true,json:async()=>({rtt:0,kind:'tcp_connect'})}));
  assert.equal(await ui.context.checkPing(),0);
  assert.match(ui.context.pingValue.title,/TCP/);
});
test('GUI API failure is not labelled server unavailable',async()=>{
  const ui=boot(async()=>{throw new Error('fixture API failure')});
  assert.equal(await ui.context.checkPing(),-1);
  assert.equal(ui.context.pingValue.textContent,'Ошибка проверки');
  assert.match(ui.context.pingValue.title,/fixture API failure/);
});
