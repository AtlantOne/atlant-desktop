const {test}=require('node:test');
const assert=require('node:assert/strict');
const {summary}=require('../../client/windows/gui/web/usage.js');
test('fractional quota has remaining bytes and bounded progress',()=>{
 const s=summary({upload:20,download:30,total:100});
 assert.equal(s.percent,50);assert.ok(s.text.includes('50'));assert.equal(s.finite,true);
 assert.equal(summary({upload:999,download:999,total:100}).percent,100);
 assert.ok(summary({upload:999,total:100}).text.includes('Осталось 0'));
});
test('unlimited and invalid counters never produce NaN or a fake full meter',()=>{
 assert.equal(summary({total:0,upload:20}).finite,false);
 assert.equal(summary({total:100,upload:-10,download:NaN}).percent,0);
 assert.ok(!summary({total:100,upload:Infinity}).text.includes('NaN'));
});
