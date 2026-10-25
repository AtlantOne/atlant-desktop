const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const source = fs.readFileSync(path.join(__dirname, '../../client/windows/gui/web/app.js'), 'utf8');
const code = source.slice(source.indexOf("const btnPasteClipboard ="), source.indexOf('// Ping All Servers'));

function setup(readText, fetch) {
  let click;
  let loads = 0;
  const messages = [];
  const button = {disabled: false, addEventListener: (_, handler) => {click = handler;}};
  vm.runInNewContext(code, {
    document: {getElementById: () => button}, navigator: {clipboard: {readText}}, fetch,
    tr: (text, n) => text.replace('{0}', n), showToast: text => messages.push(text),
    loadConfig: async () => {loads++;},
    prompt: () => {throw new Error('Unexpected manual-paste prompt');}
  });
  return {click: () => click(), button, messages, loads: () => loads};
}
const response = (body, ok = true) => ({ok, json: async () => body, text: async () => String(body)});

test('denied browser clipboard uses authenticated native POST and imports Unicode', async () => {
  const calls = [];
  const app = setup(async () => {throw new Error('permission denied');}, async (url, options) => {
    calls.push([url, options]);
    return url.includes('/clipboard/') ? response({text: ' https://例子.测试/sub\r\n '}) : response({success: true, added_count: 2});
  });
  await app.click();
  assert.equal(calls.length, 2);
  assert.equal(calls[0][0], '/api/clipboard/text');
  assert.equal(calls[0][1].method, 'POST');
  assert.deepEqual(JSON.parse(calls[1][1].body), {text: 'https://例子.测试/sub'});
  assert.equal(app.loads(), 1);
  assert.equal(app.button.disabled, false);
});

test('HTTP import errors do not reopen paste or repeat mutation', async () => {
  let calls = 0;
  const app = setup(async () => 'https://example.test/sub', async () => {calls++; return response('Subscription HTTP 503', false);});
  await app.click();
  assert.equal(calls, 1);
  assert.equal(app.loads(), 0);
  assert.match(app.messages.at(-1), /503/);
  assert.equal(app.button.disabled, false);
});

test('empty clipboard does not import; duplicate count stays zero', async () => {
  const empty = setup(async () => '  ', async () => {throw new Error('unexpected import');});
  await empty.click();
  assert.equal(empty.loads(), 0);
  const duplicate = setup(async () => 'atlant://example', async () => response({success: true, added_count: 0}));
  await duplicate.click();
  assert.match(duplicate.messages.at(-1), /0/);
});

test('busy clipboard error is displayed; button is re-enabled', async () => {
  const app = setup(async () => {throw new Error('denied');}, async () => response('clipboard is busy', false));
  await app.click();
  assert.match(app.messages.at(-1), /clipboard is busy/);
  assert.equal(app.button.disabled, false);
});

test('rapid double click imports once', async () => {
  let resolveRead;
  let calls = 0;
  const app = setup(() => new Promise(resolve => {resolveRead = resolve;}), async () => {calls++; return response({success: true, added_count: 1});});
  const first = app.click();
  await app.click();
  resolveRead('atlant://example');
  await first;
  assert.equal(calls, 1);
});
