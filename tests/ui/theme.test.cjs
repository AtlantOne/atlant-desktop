const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const script = fs.readFileSync(path.join(__dirname, '../../client/windows/gui/web/theme.js'), 'utf8');
function boot({saved=null, system=false, denied=false}={}) {
  const attributes = new Map(), buttonAttributes = new Map(), listeners = {};
  const selectListeners = {}, mediaListeners = {};
  const select = {value:'system',addEventListener(k,v){selectListeners[k]=v}};
  const media = {matches:system,addEventListener(k,v){mediaListeners[k]=v}};
  const storage = new Map(saved === null ? [] : [['atlant-theme', saved]]);
  const button = {classList:{toggle(){}},setAttribute(k,v){buttonAttributes.set(k,v)},addEventListener(k,v){listeners[k]=v}};
  vm.runInNewContext(script, {
    document:{readyState:'complete', documentElement:{setAttribute(k,v){attributes.set(k,v)},removeAttribute(k){attributes.delete(k)},getAttribute(k){return attributes.get(k)}},getElementById(id){return id==='settings-theme' ? select : button}},
    matchMedia(){return media},
    localStorage:{getItem(k){if(denied)throw Error('storage blocked');return storage.get(k)},setItem(k,v){if(denied)throw Error('storage blocked');storage.set(k,v)}}
  });
  return {attributes,buttonAttributes,storage,select,click(){listeners.click()},choose(value){select.value=value;selectListeners.change()},system(value){media.matches=value;mediaListeners.change()}};
}
test('system default and explicit preference',()=>{
  assert.equal(boot({system:true}).attributes.get('data-theme'),'dark');
  assert.equal(boot({saved:'light',system:true}).attributes.has('data-theme'),false);
  assert.equal(boot({saved:'dark'}).attributes.get('data-theme'),'dark');
  assert.equal(boot({saved:'unknown',system:true}).attributes.get('data-theme'),'dark');
});
test('toggle persists and updates accessibility state',()=>{
  const ui=boot();ui.click();
  assert.equal(ui.storage.get('atlant-theme'),'dark');
  assert.equal(ui.buttonAttributes.get('aria-checked'),'true');
  assert.equal(boot({saved:ui.storage.get('atlant-theme')}).attributes.get('data-theme'),'dark');
  ui.click();assert.equal(ui.storage.get('atlant-theme'),'light');
});
test('blocked local storage does not break appearance controls',()=>{
  const ui=boot({denied:true});assert.doesNotThrow(()=>ui.click());
  assert.equal(ui.attributes.get('data-theme'),'dark');
});
test('system theme follows OS changes; explicit theme does not',()=>{
  const ui=boot();
  assert.equal(ui.select.value,'system');
  ui.system(true); assert.equal(ui.attributes.get('data-theme'),'dark');
  ui.choose('light'); ui.system(true); assert.equal(ui.attributes.has('data-theme'),false);
  ui.choose('system'); assert.equal(ui.attributes.get('data-theme'),'dark');
  ui.system(false); assert.equal(ui.attributes.has('data-theme'),false);
});
