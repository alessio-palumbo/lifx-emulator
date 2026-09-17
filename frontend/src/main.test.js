import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import vm from 'node:vm';
import {Window} from 'happy-dom';

const source = (await readFile(new URL('./main.js',import.meta.url),'utf8')).replace(/^import .*;\n/gm,'');
const tick = () => new Promise(resolve => setImmediate(resolve));
async function app() {
  const window = new Window();
  const document = window.document;
  document.body.innerHTML = '<div id="app"></div>';
  window.HTMLCanvasElement.prototype.getContext = () => new Proxy({}, {get:(_,key)=>key==='fillStyle'?'#fff':()=>{},set:()=>true});
  const devices = ['First light','Second light'].map((Label,index)=>({Serial:`02000000000${index+1}`,Label,Model:'Color',Enabled:true,Product:27,Kind:'single_zone',Power:65535,Colors:[{}],Surface:{}}));
  const Recent = Array.from({length:80},(_,index)=>({At:new Date(index*1000).toISOString(),Direction:'RX',Target:devices[0].Serial,Type:101,TypeName:'LightGet',Replies:1,Sequence:index}));
  const state = {Devices:devices,Recent,Interfaces:['0.0.0.0'],Listening:'0.0.0.0:56700',Transport:{Received:800,Decoded:799,Replies:700,Invalid:1}};
  window.go = {app:{App:{Products:async()=>[{ID:27,Name:'Color'}],Snapshot:async()=>state,RequestLANAccess:async()=>{},Update:async()=>{}}}};
  let frame;
  window.runtime = {EventsOn:(_,callback)=>{frame=callback;}};
  const timers = new Map();
  let nextTimer = 0;
  vm.runInNewContext(source, {
    window,document,cssColor:()=> '#fff',enhanceSelects:()=>{},
    setTimeout:(fn,delay)=>{const id=++nextTimer;timers.set(id,{fn,delay});return id;},
    clearTimeout:id=>timers.delete(id),
  });
  await tick();
  assert.equal(document.querySelector('#error').textContent,'');
  return {window,document,state,frame:next=>frame(next),runTimers:delay=>{for(const [id,timer] of timers){if(timer.delay===delay){timers.delete(id);timer.fn();}}},close:()=>window.happyDOM.abort()};
}
test('buttons show feedback for quick clicks and LAN completion expires', async () => {
  const a = await app();
  try {
    const button = a.document.querySelector('#lan-access');
    button.click();
    assert.equal(button.classList.contains('clicked'),true);
    await tick();
    assert.match(a.document.querySelector('#network-note').textContent,/request completed/);
    a.runTimers(220);
    assert.equal(button.classList.contains('clicked'),false);
    a.runTimers(6000);
    assert.equal(a.document.querySelector('#network-note').textContent,'');
  } finally {await a.close();}
});
test('identity forms dismiss outside and only one device can be edited', async () => {
  const a = await app();
  try {
    const cards = [...a.document.querySelectorAll('article')];
    const edit = index => cards[index].querySelector('.edit').click();
    const form = index => cards[index].querySelector('.identity');
    edit(0);
    assert.equal(form(0).hidden,false);
    form(0).querySelector('input').dispatchEvent(new a.window.PointerEvent('pointerdown',{bubbles:true}));
    assert.equal(form(0).hidden,false);
    edit(1);
    assert.equal(form(0).hidden,true);
    assert.equal(form(1).hidden,false);
    a.document.querySelector('header').dispatchEvent(new a.window.PointerEvent('pointerdown',{bubbles:true}));
    assert.equal(form(1).hidden,true);
    edit(0);
    form(0).querySelector('.cancel').click();
    assert.equal(form(0).hidden,true);
  } finally {await a.close();}
});
test('traffic uses two counter lines and the existing bounded history', async () => {
  const a = await app();
  try {
    const lines = a.document.querySelectorAll('#transport>span');
    assert.equal(lines.length,2);
    assert.match(lines[0].textContent,/RX 800.*TX 700/);
    assert.match(lines[1].textContent,/Dropped 1.*send errors 0/);
    const history = a.document.querySelector('#activity');
    assert.equal(history.children.length,80);
    assert.equal(history.firstChild.dataset.key.split('|')[0],a.state.Recent.at(-1).At);
    const frozenHTML=history.innerHTML;
    history.scrollTop=65;
    history.dispatchEvent(new a.window.Event('scroll'));
    const mode=a.document.querySelector('#traffic-mode');
    assert.equal(mode.textContent,'Resume live');
    assert.equal(mode.getAttribute('aria-pressed'),'true');
    let latest;
    // Inspection remains stable even after every original row ages out of Go's buffer.
    for(let index=80;index<180;index++){
      latest={...a.state,Transport:{Received:900},Recent:Array.from({length:80},(_,offset)=>({...a.state.Recent[0],At:new Date((index-79+offset)*1000).toISOString()}))};
      a.frame(latest);
    }
    assert.equal(history.innerHTML,frozenHTML);
    assert.equal(history.scrollTop,65);
    assert.match(a.document.querySelector('#transport').textContent,/RX 900/);
    mode.click();
    assert.equal(mode.textContent,'Pause');
    assert.equal(mode.getAttribute('aria-pressed'),'false');
    assert.equal(history.scrollTop,0);
    assert.equal(history.children.length,80);
    assert.equal(history.firstChild.dataset.key.split('|')[0],latest.Recent.at(-1).At);
    mode.click();
    const manuallyPausedHTML=history.innerHTML;
    a.frame({...latest,Recent:[...latest.Recent.slice(1),{...latest.Recent[0],At:new Date(200000).toISOString()}]});
    assert.equal(history.innerHTML,manuallyPausedHTML);
  } finally {await a.close();}
});
