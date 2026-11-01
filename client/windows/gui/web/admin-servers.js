'use strict';
(() => {
  const nav = document.createElement('button'); nav.type = 'button'; nav.className = 'nav-item'; nav.textContent = tr("Мои серверы"); nav.dataset.tab = 'tab-admin-servers';
  const navIcon=document.createElementNS('http://www.w3.org/2000/svg','svg');navIcon.setAttribute('viewBox','0 0 24 24');navIcon.setAttribute('fill','none');navIcon.setAttribute('stroke','currentColor');navIcon.setAttribute('stroke-width','1.7');navIcon.setAttribute('aria-hidden','true');
  for(const y of [3,14]){const rect=document.createElementNS('http://www.w3.org/2000/svg','rect');for(const[key,value]of Object.entries({x:3,y,width:18,height:7,rx:2}))rect.setAttribute(key,value);navIcon.append(rect);}
  const navLabel=document.createElement('span');navLabel.textContent=tr('Мои серверы');nav.replaceChildren(navIcon,navLabel);
  document.querySelector('.nav-menu').append(nav);
  const section = document.createElement('section'); section.className = 'tab-pane'; section.id = 'tab-admin-servers';
  document.querySelector('.tab-pane').parentElement.append(section);
  let catalog = [], selected = '', busy=false;
  const heading=document.createElement('div');heading.className='admin-heading';section.append(heading);
  const title = document.createElement('h2'); title.textContent = tr("Мои серверы"); heading.append(title);
  const list = document.createElement('div'); list.className='admin-server-list'; section.append(list);
  const editor=document.createElement('dialog');editor.className='admin-editor';document.body.append(editor);
  const editorTitle=document.createElement('h2');editorTitle.textContent=tr('Добавить сервер');editor.append(editorTitle);
  editorTitle.id='admin-editor-title';editor.setAttribute('aria-labelledby',editorTitle.id);
  const form = document.createElement('form'); form.className = 'admin-form'; editor.append(form);
  const advanced=document.createElement('details');advanced.className='admin-advanced';const summary=document.createElement('summary');summary.textContent=tr('Дополнительно');advanced.append(summary);
  const fields = {};
  for (const [key, label, value, type] of [
    ['name',tr("Название"),'','text'], ['host',tr("IP или домен"),'','text'], ['port',tr("SSH-порт"),'22','number'],
    ['username',tr("SSH-логин"),'root','text'], ['password',tr("SSH-пароль"),'','password'],
    ['vpn_port',tr("Порт VPN"),'1026','number'], ['panel_port',tr("Локальный порт панели на сервере"),'19090','number'],
    ['fingerprint',tr("Подтверждённый SSH-отпечаток"),'','text']
  ]) {
    const row = document.createElement('label'); row.className = 'settings-row'; const text = document.createElement('span'); text.textContent = label;
    const input = document.createElement('input'); input.type = type; input.value = value; input.className = 'settings-input-text'; input.autocomplete = type === 'password' ? 'new-password' : 'off';
    if (key === 'fingerprint') input.readOnly = true;
    fields[key] = input; row.append(text,input);row.className='admin-field';
    (['name','port','vpn_port','panel_port','fingerprint'].includes(key)?advanced:form).append(row);
  }
  for (const key of ['host','port']) fields[key].addEventListener('input',()=>{ fields.fingerprint.value=''; });
  form.append(advanced);
  const status = document.createElement('p');status.className='admin-status';status.setAttribute('role','status');section.append(status);
  const editorStatus=document.createElement('p');editorStatus.className='admin-status';editor.append(editorStatus);
  const observer=new MutationObserver(()=>{editorStatus.textContent=status.textContent;});observer.observe(status,{childList:true,characterData:true,subtree:true});
  const request = () => Object.fromEntries(Object.entries(fields).filter(([k])=>k!=='name').map(([k,v])=>[k,['port','vpn_port','panel_port'].includes(k)?Number(v.value):v.value]));
  async function api(op, body={}) { const response=await fetch('/api/admin/'+op,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}); const data=await response.json(); if (!response.ok) throw Error(data.error||tr("Операция не выполнена")); return data; }
  function button(label,action) { const b=document.createElement('button');b.type='button';b.className='btn-secondary';b.textContent=label;b.addEventListener('click',async()=>{if(busy)return;b.disabled=true;try{await action();}catch(e){status.textContent=e.message;}finally{b.disabled=false;}});form.append(b);return b; }
  function fill(server){selected=server?.id||'';for(const[key,input]of Object.entries(fields))input.value=server?(key==='name'?server.name:server.request[key]??''):({username:'root',port:22,vpn_port:1026,panel_port:19090}[key]??'');fields.password.value='';fields.password.placeholder=server?.has_credentials?tr('Пароль сохранён'):'';advanced.open=false;status.textContent='';}
  function configureMode(existing){primaryAction.textContent=tr(existing?'Открыть':'Установить Atlant');primaryAction.dataset.install=existing?'false':'true';updateAction.hidden=!existing;}
  function openEditor(server){fill(server);configureMode(!!server);editorTitle.textContent=tr(server?'Изменить сервер':'Добавить сервер');editor.showModal();fields.host.focus();}
  function cardButton(parent,label,run,allowBusy=false){const b=document.createElement('button');b.type='button';b.className='btn-secondary';b.textContent=tr(label);b.onclick=async()=>{if(busy&&!allowBusy)return;b.disabled=true;try{await run();}catch(e){status.textContent=e.message;}finally{b.disabled=false;}};parent.append(b);return b;}
  async function refresh() {
    catalog=await api('list');list.replaceChildren();addServer.hidden=!catalog.length;
    if(!catalog.length){const empty=document.createElement('div');empty.className='admin-empty';const mark=document.createElement('div');mark.className='admin-server-icon';mark.textContent='＋';const caption=document.createElement('h3');caption.textContent=tr('Добавьте свой сервер');empty.append(mark,caption);cardButton(empty,'Добавить сервер',()=>openEditor(null)).classList.add('admin-primary');list.append(empty);return;}
    for(const server of catalog){
      const card=document.createElement('article');card.className='admin-server-card';const mark=document.createElement('span');mark.className='admin-server-icon';mark.textContent='▤';const info=document.createElement('div');info.className='admin-server-info';const name=document.createElement('h3');name.textContent=server.name;const address=document.createElement('p');address.textContent=server.request.host;info.append(name,address);card.append(mark,info);
      cardButton(card,'Открыть',async()=>{fill(server);if(server.has_credentials||server.active_job)await begin(false);else{configureMode(true);editorTitle.textContent=tr('Подключение к серверу');editor.showModal();fields.password.focus();}});
      const menu=document.createElement('details');menu.className='admin-server-menu';const toggle=document.createElement('summary');toggle.textContent='⋯';toggle.setAttribute('aria-label',tr('Действия сервера'));menu.append(toggle);const actions=document.createElement('div');actions.className='admin-menu-items';menu.append(actions);
      cardButton(actions,'Изменить',()=>{menu.open=false;openEditor(server);});
      cardButton(actions,'Обновить Atlant',()=>{menu.open=false;openEditor(server);});
      cardButton(actions,'Закрыть SSH-канал',async()=>{if(server.active_job)await api('close',{job:server.active_job});await refresh();},true);
      cardButton(actions,'Удалить из списка',async()=>{if(confirm(tr('Удалить сервер из приложения? Данные на VDS останутся.'))){await api('remove',{id:server.id});await refresh();}}).classList.add('admin-danger');
      card.append(menu);list.append(card);
    }
  }
  const addServer=cardButton(heading,'Добавить сервер',()=>openEditor(null));addServer.classList.add('admin-primary');
  const saveAction=button(tr("Сохранить сервер"),async()=>{const data=await api('save',{id:selected,name:fields.name.value,request:request(),remember:true});selected=data.id;await refresh();editor.close();status.textContent='';});advanced.append(saveAction);
  async function begin(install) {
    if(busy)return;busy=true;form.querySelectorAll('input,button').forEach(el=>el.disabled=true);
    status.textContent=tr('SSH-подключение');
    try {
    if(!fields.host.value.trim())throw Error(tr('Укажите адрес сервера'));
    if(!fields.fingerprint.value){const data=await api('probe',{request:request()});if(!confirm(tr('Сверьте SSH-отпечаток с консолью провайдера:\n')+data.fingerprint+tr('\nДоверять этому ключу?')))return;fields.fingerprint.value=data.fingerprint;}
    if(install&&!confirm(tr('Установить или обновить Atlant? Существующая установка будет обновлена с сохранением пользователей, подписок, ключей и настроек. Другие службы не изменяются.')))return;
    const saved=await api('save',{id:selected,name:fields.name.value||fields.host.value,request:request(),remember:true});selected=saved.id;await refresh();
    const data=await api('begin',{id:selected,request:{...request(),install,install_docker:install}});const operationJob=data.job;fields.password.value='';editor.close();await refresh();
    for(;;){const current=await api('status',{job:operationJob});status.textContent=({connecting:tr("SSH-подключение"),preparing:tr("Подготовка релиза"),installing:tr("Загрузка готового релиза"),updating:tr('Обновление Atlant'),opening:tr("Подключение панели"),ready:tr("Панель готова"),failed:tr("Ошибка"),closed:tr('Подключение закрыто'),missing:tr("Подключение закрыто"),stopped:tr('Контейнер остановлен')})[current.phase]||current.phase;
      if(current.phase==='failed')throw Error(current.error);if(current.phase==='ready'){status.textContent='';heading.hidden=true;list.hidden=true;await AtlantAdminConsole.open(operationJob,fields.name.value||fields.host.value,section,()=>{heading.hidden=false;list.hidden=false;refresh().catch(e=>status.textContent=e.message);});break;}if(['missing','closed','stopped'].includes(current.phase))break;await new Promise(r=>setTimeout(r,1500));}
    } finally {busy=false;form.querySelectorAll('input,button').forEach(el=>el.disabled=false);}
  }
  const primaryAction=button(tr('Установить Atlant'),()=>begin(primaryAction.dataset.install!=='false'));primaryAction.classList.add('admin-primary');
  const updateAction=button(tr('Обновить Atlant'),()=>begin(true));updateAction.hidden=true;
  cardButton(editor,'Закрыть',()=>editor.close());
  editor.addEventListener('cancel',event=>{if(busy)event.preventDefault();});
  nav.addEventListener('click',()=>{document.querySelectorAll('.nav-item,.tab-pane').forEach(n=>n.classList.remove('active'));nav.classList.add('active');section.classList.add('active');refresh().catch(e=>status.textContent=e.message);});
  document.querySelectorAll('.nav-item').forEach(item=>{if(item!==nav)item.addEventListener('click',()=>{nav.classList.remove('active');section.classList.remove('active');});});
  form.addEventListener('submit',e=>e.preventDefault());
})();
