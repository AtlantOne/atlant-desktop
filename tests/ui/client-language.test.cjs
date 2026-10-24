const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {resolveLanguage, format, translate} = require('../../client/windows/gui/web/i18n.js');
const catalog = JSON.parse(fs.readFileSync(path.join(__dirname, '../../client/windows/gui/web/client-languages.json'), 'utf8'));
test('language resolution handles regional locales and invalid preferences', () => {
  assert.equal(resolveLanguage('system', 'ru-RU'), 'ru');
  assert.equal(resolveLanguage('system', 'zh-CN'), 'zh');
  assert.equal(resolveLanguage('en', 'ru-RU'), 'en');
  assert.equal(resolveLanguage('invalid', 'de-DE'), 'en');
});
test('all languages translate explicit UI keys', () => {
  assert.equal(translate(catalog, 'en', 'Настройки'), 'Settings');
  assert.equal(translate(catalog, 'zh', 'Настройки'), '设置');
  assert.equal(translate(catalog, 'ru', 'Настройки'), 'Настройки');
});
test('formatting is one-pass and never interprets user-supplied placeholders', () => {
  assert.equal(format('{0} {1}', ['{1}', 'literal']), '{1} literal');
  assert.equal(translate(catalog, 'en', 'Действия сервера {0}', '<server>{0}'), 'Server actions: <server>{0}');
  assert.equal(translate(catalog, 'zh', 'my server name'), 'my server name');
});
