const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../../client/windows/gui/web/app.js'), 'utf8');
const dashboard = source.slice(source.indexOf('// Update Dashboard Card'), source.indexOf('// Render Grouped Providers'));

function render(cfg) {
  const elements = Object.fromEntries(['card-server-name', 'card-server-meta', 'dashboard-transport', 'dashboard-mode']
    .map(id => [id, {textContent: ''}]));
  vm.runInNewContext(dashboard, {cfg, tr: value => value, document: {getElementById: id => elements[id]}});
  return elements;
}

test('dashboard does not display an unselected first profile as active', () => {
  const ui = render({servers: [{id: 'first', name: 'Not selected', server: 'first.example:443'}]});
  assert.notEqual(ui['card-server-name'].textContent, 'Not selected');
  assert.equal(ui['dashboard-transport'].textContent, 'НЕ ВЫБРАН');
});

test('dashboard shows the selected profile rather than the first entry', () => {
  const ui = render({active_server_id: 'second', servers: [
    {id: 'first', name: 'First', server: 'first.example:443'},
    {id: 'second', name: 'Selected', server: 'second.example:443', transport: 'quic'},
  ]});
  assert.equal(ui['card-server-name'].textContent, 'Selected');
  assert.equal(ui['dashboard-transport'].textContent, 'QUIC');
});
