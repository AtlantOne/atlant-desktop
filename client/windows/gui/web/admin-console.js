'use strict';
// This view belongs to the desktop client. Only JSON operations cross the SSH bridge.
window.AtlantAdminConsole = (() => {
  const el=(tag,text,cls)=>{const node=document.createElement(tag);if(text!==undefined)node.textContent=text;if(cls)node.className=cls;return node;};
  let generation=0;
  async function open(job,name,parent,onBack){
    const version=++generation, root=el('div',undefined,'managed-console');parent.append(root);
    const heading=el('header',undefined,'admin-heading'),title=el('div');title.append(el('h2',name),el('small',tr('Управление сервером')));heading.append(title);root.append(heading);
    const tabs=el('div',undefined,'managed-tabs'),notice=el('p','','admin-status'),content=el('div');root.append(tabs,notice,content);
    let page='subscriptions',render=0,timer;
    const visibility=new MutationObserver(()=>{if(!parent.classList.contains('active'))clearTimeout(timer);else if(page==='logs')load();});visibility.observe(parent,{attributes:true,attributeFilter:['class']});
    function button(parent,label,run,danger=false){const b=el('button',tr(label),'btn-secondary'+(danger?' admin-danger':''));b.type='button';b.onclick=async()=>{b.disabled=true;try{await run();}catch(e){notice.textContent=e.message;}finally{b.disabled=false;}};parent.append(b);return b;}
    button(heading,'Назад к моим серверам',()=>{generation++;clearTimeout(timer);visibility.disconnect();root.remove();onBack();});
    async function call(command,reveal=false,confirmed=false){
      const response=await fetch('/api/admin/command',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({job,command,reveal,confirmed})});
      const data=await response.json();if(!response.ok)throw Error(data.error==='group_changed'?tr('Состав группы изменился. Обновите её перед выполнением действия.'):data.error?.startsWith('group_partial_update')?tr('Часть участников уже обновлена. Обновите группу перед повторной попыткой.'):data.error||tr('Операция не выполнена'));
      if(data.confirmation_required){if(!confirm(tr('Подтвердить изменение')+'?'))return null;return call(command,reveal,true);}return data.result;
    }
    async function mutate(command,reveal=false){await call(command,reveal);await load();}
    function form(title,fields,save){
      const dialog=el('dialog',undefined,'admin-editor'),body=el('form',undefined,'admin-form'),inputs={};dialog.append(el('h2',title),body);document.body.append(dialog);
      for(const field of fields){const label=el('label',undefined,'admin-field');label.append(el('span',tr(field.label)));let input;
        if(field.options){input=el('select');for(const [value,text]of field.options){const option=el('option',tr(text));option.value=value;input.append(option);}}
        else{input=el('input');input.type=field.type||'text';}
        if(field.type==='checkbox')input.checked=!!field.value;else input.value=field.value??'';
        input.required=!!field.required;input.disabled=!!field.disabled;if(input.type==='number'){input.min='0';input.step=field.step||'1';}label.append(input);body.append(label);inputs[field.key]=input;
      }
      const error=el('p','','admin-status');body.append(error);const submit=el('button',tr('Сохранить'),'btn-secondary admin-primary');submit.type='submit';body.append(submit);button(body,'Отмена',()=>dialog.close());
      body.onsubmit=async event=>{event.preventDefault();submit.disabled=true;try{const values=Object.fromEntries(Object.entries(inputs).map(([k,i])=>[k,i.type==='checkbox'?i.checked:i.value]));await save(values);dialog.close();await load();}catch(e){error.textContent=e.message;}finally{submit.disabled=false;}};
      dialog.addEventListener('close',()=>dialog.remove());dialog.showModal();return {body,inputs};
    }
    const number=(value)=>{const n=Number(value);if(!Number.isFinite(n)||n<0)throw Error(tr('Операция не выполнена'));return n;};
    const expiry=value=>{if(!value)return 0;const n=Math.floor(new Date(value).getTime()/1000);if(!Number.isFinite(n)||n<0)throw Error(tr('Некорректная дата'));return n;};
    const date=value=>{if(!value)return '';const d=new Date(value*1000);return new Date(d.getTime()-d.getTimezoneOffset()*60000).toISOString().slice(0,16);};
    const quota=bytes=>(Number(bytes||0)/1073741824).toFixed(2)+' GiB';
    function row(title,subtitle){const row=el('article',undefined,'managed-row'),info=el('div',undefined,'managed-row-info'),actions=el('div',undefined,'managed-actions');info.append(el('strong',title),el('small',subtitle));row.append(info,actions);content.append(row);return {row,info,actions};}
    async function editUser(user){form(tr(user?'Изменить':'Добавить пользователя'),[
      {key:'name',label:'Имя пользователя',value:user?.name,required:true,disabled:!!user},{key:'note',label:'Заметка',value:user?.note},
      {key:'ip_limit',label:'Лимит IP',value:user?.ip_limit||0,type:'number'},{key:'expires',label:'Срок действия',value:date(user?.expires_at),type:'datetime-local'},
      {key:'enabled',label:'Включено',value:user?.enabled??true,type:'checkbox'}],async v=>{const patch={note:v.note,ip_limit:number(v.ip_limit),expires_at:expiry(v.expires),enabled:v.enabled};await call(user?'set '+user.name+' '+JSON.stringify(patch):'usercreate '+JSON.stringify({...patch,name:v.name}));});}
    async function editSubscription(sub,catalog){
      const selected=new Set((sub?.inbound_refs||[]).filter(ref=>ref.server_id===catalog.server_id).map(ref=>ref.inbound_id));
      const {body}=form(tr(sub?'Изменить подписку':'Создать подписку'),[
        {key:'title',label:'Название',value:sub?.title,required:true},{key:'quota',label:'Лимит трафика (ГиБ)',value:Number(sub?.total_limit||0)/1073741824,type:'number',step:'any'},
        {key:'expires',label:'Срок действия',value:date(sub?.expires_at),type:'datetime-local'},
        ...(!sub||sub.managed?[{key:'ip_limit',label:'Лимит IP',value:sub?.ip_limit||0,type:'number'},{key:'note',label:'Заметка',value:sub?.note}]:[]),
        {key:'enabled',label:'Включено',value:sub?.enabled??true,type:'checkbox'}],async v=>{
          const refs=[...(sub?.inbound_refs||[]).filter(ref=>ref.server_id!==catalog.server_id),...[...selected].map(inbound_id=>({server_id:catalog.server_id,inbound_id}))];
          if(!refs.length&&!sub?.nodes?.length)throw Error(tr('Выберите подключения'));
          const patch={title:v.title,total_limit:Math.round(number(v.quota)*1073741824),expires_at:expiry(v.expires),enabled:v.enabled,inbound_refs:refs};
          if(!sub||sub.managed)Object.assign(patch,{ip_limit:number(v.ip_limit),note:v.note});
          if(sub)patch.revision=sub.revision;
          await call(sub?'subset '+sub.id+' '+JSON.stringify(patch):'subscribercreate '+JSON.stringify(patch),!sub);
        });
      const choices=el('fieldset');choices.append(el('legend',tr('Подключения')));
      for(const item of catalog.inbounds||[]){const spec=item.settings,label=el('label',undefined,'admin-remember'),input=el('input');input.type='checkbox';input.checked=selected.has(spec.id);input.onchange=()=>input.checked?selected.add(spec.id):selected.delete(spec.id);label.append(input,document.createTextNode(spec.name+' · '+spec.listen));choices.append(label);}body.prepend(choices);
    }
    function editInbound(spec,catalog){
      const selected=new Set(spec?.transports||['tcp','quic','xhttp']);
      const {body}=form(tr(spec?'Изменить подключение':'Добавить подключение'),[
        {key:'id',label:'Идентификатор',value:spec?.id||'in_'+crypto.randomUUID().replaceAll('-','').slice(0,12),required:true,disabled:!!spec},
        {key:'name',label:'Название',value:spec?.name,required:true},{key:'listen',label:'Адрес прослушивания',value:spec?.listen||':1026',required:true},
        {key:'advertise_addr',label:'Адрес VPN для клиентов (IP/домен:порт)',value:spec?.advertise_addr,required:true},
        {key:'sni',label:'Имя сервера REALITY (SNI)',value:spec?.sni,required:true},{key:'enabled',label:'Включено',value:spec?.enabled??true,type:'checkbox'}],async v=>{
          if(!selected.size)throw Error(tr('Выберите транспорт'));
          await call((spec?'inboundset ':'inboundcreate ')+catalog.revision+' '+JSON.stringify({...spec,...v,wire_version:7,transports:[...selected]}));
        });
      for(const transport of ['tcp','quic','xhttp']){const label=el('label',undefined,'admin-remember'),input=el('input');input.type='checkbox';input.checked=selected.has(transport);input.onchange=()=>input.checked?selected.add(transport):selected.delete(transport);label.append(input,document.createTextNode(transport.toUpperCase()));body.prepend(label);}
    }
    async function share(command,title){const data=await call(command,true);const value=data?.url||data?.link;if(!value)throw Error(tr('Ссылка недоступна'));AtlantShare.open(value,title,{copy:tr('Копировать'),close:tr('Закрыть'),link:tr('Ссылка'),qr:tr('QR-код')});}
    async function load(){
      clearTimeout(timer);const token=++render;notice.textContent='';
      try{
        let data,catalog,accounts;
        if(page==='subscriptions'){[data,catalog]=await Promise.all([call('subscriptions'),call('inbounds')]);}
        else if(page==='groups'){[data,accounts]=await Promise.all([call('groups'),call('usersall')]);}
        else data=await call({users:'usersall',inbounds:'inbounds',groups:'groups',logs:'logs 50',settings:'configinfo',overview:'status'}[page]);
        if(version!==generation||token!==render)return;content.replaceChildren();
        const toolbar=el('div',undefined,'admin-heading');content.append(toolbar);
        if(page==='subscriptions'){
          button(toolbar,'Создать подписку',()=>editSubscription(null,catalog));
          for(const sub of data||[]){const q=sub.global_quota,used=q?Number(q.upload||0)+Number(q.download||0):Number(sub.up_bytes||0)+Number(sub.down_bytes||0),limit=Number(q?.total_limit??sub.total_limit??0);const r=row(sub.title,quota(used)+(limit?' / '+quota(limit):' · '+tr('Безлимит')));
            if(limit){const progress=el('progress');progress.max=limit;progress.value=used;r.info.append(progress);}
            const connections=el('details'),summary=el('summary',tr('Подключения')+' · '+((sub.inbound_refs||[]).length+(sub.nodes||[]).length));connections.append(summary);
            for(const ref of sub.inbound_refs||[]){const spec=catalog.inbounds?.find(item=>catalog.server_id===ref.server_id&&item.settings.id===ref.inbound_id)?.settings;connections.append(el('small',spec?spec.name+' · '+spec.advertise_addr:ref.inbound_id));}
            for(const node of sub.nodes||[])connections.append(el('small',node.name+' · '+node.server));r.info.append(connections);
            button(r.actions,'Изменить',()=>editSubscription(sub,catalog));button(r.actions,'Ссылка',()=>share('subshare '+sub.id,sub.title));button(r.actions,'Удалить',()=>mutate('subdelete '+sub.id),true);}
        }else if(page==='users'){
          button(toolbar,'Добавить пользователя',()=>editUser(null));for(const user of data.users||[]){const r=row(user.name,(user.enabled?tr('Включено'):tr('Отключено'))+(user.note?' · '+user.note:''));button(r.actions,'Изменить',()=>editUser(user));button(r.actions,user.enabled?'Отключить':'Включить',()=>mutate((user.enabled?'disable ':'enable ')+user.name));button(r.actions,'Отключить сессии',()=>mutate('kick '+user.name),true);button(r.actions,'Удалить',()=>mutate('delete '+user.name),true);}
        }else if(page==='inbounds'){
          button(toolbar,'Добавить подключение',()=>editInbound(null,data));button(toolbar,'Применить подключения',()=>mutate('inboundapply '+data.revision));for(const item of data.inbounds||[]){const spec=item.settings,r=row(spec.name,spec.listen+' · '+(spec.transports||[]).join(' / ').toUpperCase());button(r.actions,'Изменить',()=>editInbound(spec,data));button(r.actions,'Удалить',()=>mutate('inbounddelete '+data.revision+' '+spec.id),true);}
        }else if(page==='groups'){
          async function editGroup(group){
            const selected=new Set(group?.users||[]),all=accounts||await call('usersall');
            const {body}=form(tr('Группа'),[{key:'name',label:'Название',value:group?.name,required:true},{key:'note',label:'Заметка',value:group?.note}],async values=>{await call(group?'groupset '+group.id+' '+JSON.stringify({...values,users:[...selected],revision:group.revision}):'groupcreate '+JSON.stringify({...values,users:[...selected]}));});
            const choices=el('fieldset'),legend=el('legend',tr('Участники группы')),search=el('input'),list=el('div'),count=el('small');search.type='search';search.placeholder=tr('Поиск');choices.append(legend,search,count,list);body.insertBefore(choices,body.querySelector('.admin-status'));let visible=100;
            function draw(){count.textContent=tr('Выбрано')+': '+selected.size;list.replaceChildren();const filtered=all.users.filter(user=>user.name.toLowerCase().includes(search.value.toLowerCase()));for(const user of filtered.slice(0,visible)){const label=el('label',undefined,'admin-remember'),input=el('input');input.type='checkbox';input.checked=selected.has(user.name);input.onchange=()=>{input.checked?selected.add(user.name):selected.delete(user.name);count.textContent=tr('Выбрано')+': '+selected.size;};label.append(input,document.createTextNode(user.name));list.append(label);}if(filtered.length>visible)button(list,'Показать ещё',()=>{visible+=100;draw();});}
            search.oninput=()=>{visible=100;draw();};draw();
          }
          button(toolbar,'Добавить группу',()=>editGroup(null));for(const group of data||[]){const members=(accounts.users||[]).filter(user=>(group.users||[]).includes(user.name)),active=members.filter(user=>user.enabled).length;const r=row(group.name,members.length+' · '+tr('Включено')+': '+active+' · '+quota(Number(group.up_bytes||0)+Number(group.down_bytes||0)));
            const details=el('details'),summary=el('summary',tr('Участники группы'));details.append(summary);for(const user of members)details.append(el('small',user.name+' · '+tr(user.enabled?'Включено':'Отключено')));r.info.append(details);
            button(r.actions,'Изменить',()=>editGroup(group));for(const [label,enabled]of [['Включить всех',true],['Отключить всех',false]]){const action=button(r.actions,label,()=>mutate('groupaccess '+group.id+' '+JSON.stringify({revision:group.revision,enabled})),!enabled);action.disabled=!members.length;}button(r.actions,'Удалить',()=>mutate('groupdelete '+group.id),true);}
        }else if(page==='logs'){
          const log=el('pre',Array.isArray(data.logs)?data.logs.slice(-50).join('\n'):String(data.logs||''),'managed-log');content.append(log);if(parent.classList.contains('active'))timer=setTimeout(load,2500);
        }else if(page==='settings'){
          const level=data.log_level||'info';row(tr('Уровень логирования'),level);button(toolbar,'Изменить',()=>form(tr('Уровень логирования'),[{key:'log_level',label:'Уровень логирования',value:level,options:['debug','info','warn','error','none'].map(v=>[v,v])}],values=>call('configsave '+JSON.stringify(values))));
          content.append(el('p',tr('Изменение уровня логирования применяется после перезапуска службы VPN.'),'admin-status'));button(toolbar,'Перезапустить',()=>mutate('service restart'),true);
        }else{
          const service=await call('service status');if(token!==render||version!==generation)return;row(tr('Служба VPN'),service.active||service.state||'');for(const action of ['start','stop','restart'])button(toolbar,{start:'Запустить',stop:'Остановить',restart:'Перезапустить'}[action],()=>mutate('service '+action),action!=='start');
          for(const key of ['uptime','online','total_users','version'])if(data[key]!==undefined)row(tr({uptime:'Время работы',online:'В сети',total_users:'Пользователи',version:'Версия'}[key]),String(data[key]));
        }
        if(content.children.length===1)content.append(el('p',tr('Список пуст'),'admin-status'));
      }catch(e){if(token===render&&version===generation)notice.textContent=e.message;}
    }
    for(const [key,label]of Object.entries({subscriptions:'Подписки',users:'Пользователи',inbounds:'Подключения',groups:'Группы',overview:'Обзор',logs:'Журнал',settings:'Настройки'})){const b=button(tabs,label,()=>{page=key;for(const other of tabs.children)other.classList.toggle('selected',other===b);return load();});b.classList.toggle('selected',page===key);}
    await load();
  }
  return {open};
})();
