import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import vm from 'node:vm';
import {Window} from 'happy-dom';

const source = (await readFile(new URL('./main.js',import.meta.url),'utf8')).replace(/^import .*;\n/gm,'');
const tick = () => new Promise(resolve => setImmediate(resolve));
async function app(platform='darwin') {
  const window = new Window();
  const document = window.document;
  document.body.innerHTML = '<div id="app"></div>';
  window.HTMLCanvasElement.prototype.getContext = () => new Proxy({}, {get:(_,key)=>key==='fillStyle'?'#fff':()=>{},set:()=>true});
  const devices = ['First light','Second light'].map((Label,index)=>({Serial:`02000000000${index+1}`,Label,Model:'Color',Enabled:true,Product:27,Kind:'single_zone',Power:65535,Colors:[{}],Surface:{}}));
  const Recent = Array.from({length:80},(_,index)=>({At:new Date(index*1000).toISOString(),Direction:'RX',Target:devices[0].Serial,Type:101,TypeName:'LightGet',Replies:1,Sequence:index}));
  const state = {Platform:platform,Location:{ID:'11111111-1111-4111-8111-111111111111',Label:'Test lab'},Group:{ID:'22222222-2222-4222-8222-222222222222',Label:'Alice'},Devices:devices,Recent,Interfaces:['0.0.0.0','192.168.1.10'],Listening:'0.0.0.0:56700',Transport:{Received:800,Decoded:799,Replies:700,Invalid:1}};
  window.go = {app:{App:{Products:async()=>[{ID:27,Name:'Color'},{ID:55,Name:'Tile',Matrix:true,Chain:true},{ID:219,Name:'Luna',Matrix:true,Chain:false}],Snapshot:async()=>state,ListenOn:async ip=>{state.Listening=ip+':56700';},RequestLANAccess:async()=>{},UpdateMembership:async(locationLabel,locationID,groupLabel,groupID)=>{state.Location={Label:locationLabel,ID:locationID};state.Group={Label:groupLabel,ID:groupID};},Update:async()=>{}}}};
  let frame;
  window.runtime = {EventsOn:(_,callback)=>{frame=callback;}};
  const timers = new Map();
  let nextTimer = 0;
  vm.runInNewContext(source, {
    window,document,appIcon:'/test-appicon.png',cssColor:()=> '#fff',enhanceSelects:()=>{},createPresentation:()=>({openDevice:()=>{},openAll:()=>{},render:()=>{}}),
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

test('location/group edits survive frames and save with unchanged IDs',async()=>{
 const a=await app();
 try{
  const input=a.document.querySelector('#group-label');
  assert.equal(input.value,'Alice');
  const id=a.document.querySelector('#group-id').value;
  input.value='Bob';input.dispatchEvent(new a.window.Event('input',{bubbles:true}));
  a.frame({...a.state});
  assert.equal(input.value,'Bob');
  a.document.querySelector('#membership').dispatchEvent(new a.window.Event('submit',{bubbles:true,cancelable:true}));
  await tick();
  assert.equal(a.state.Group.Label,'Bob');
  assert.equal(a.state.Group.ID,id);
  assert.match(a.document.querySelector('#membership-summary').textContent,/Bob/);
 }finally{await a.close();}
});

test('interface selection applies immediately and restores the actual selection on failure',async()=>{
 const a=await app();
 try{
  assert.equal(a.document.querySelector('#rebind'),null);
  assert.equal(a.document.body.textContent.includes('State and animations are evaluated in Go.'),false);
  const select=a.document.querySelector('#interface');
  select.value='192.168.1.10';select.dispatchEvent(new a.window.Event('change'));
  assert.equal(select.disabled,true);
  await tick();
  assert.equal(a.state.Listening,'192.168.1.10:56700');
  assert.equal(select.disabled,false);
  assert.match(a.document.querySelector('#status').textContent,/192\.168\.1\.10/);
  let calls=0;
  a.window.go.app.App.ListenOn=async()=>{calls++;throw new Error('Cannot bind interface');};
  select.dispatchEvent(new a.window.Event('change'));
  await tick();assert.equal(calls,0);
  select.value='0.0.0.0';select.dispatchEvent(new a.window.Event('change'));
  await tick();assert.equal(calls,1);
  assert.equal(select.value,'192.168.1.10');
  assert.equal(select.disabled,false);
  assert.match(a.document.querySelector('#error').textContent,/Cannot bind interface/);
  assert.match(a.document.querySelector('#lan-access').title,/macOS.*Sends no packet/);
 }finally{await a.close();}
});

test('LAN access helper is visible only on macOS',async()=>{
 for(const platform of ['darwin','linux','windows']){
  const a=await app(platform);
  try{assert.equal(a.document.querySelector('#lan-access').hidden,platform!=='darwin');}
  finally{await a.close();}
 }
});
test('LAN panel shows membership names and cog settings dismiss outside or with Escape',async()=>{
 const a=await app();
 try{
  const panel=a.document.querySelector('#network-settings-panel');
  const toggle=a.document.querySelector('#network-settings-toggle');
  assert.equal(panel.hidden,true);
  assert.match(a.document.querySelector('.network #membership-summary').textContent,/Location: Test lab.*Group: Alice/);
  assert.equal(a.document.querySelector('aside #membership'),null);
  toggle.click();assert.equal(panel.hidden,false);
  assert.equal(toggle.getAttribute('aria-expanded'),'true');
  assert.equal(a.document.activeElement.id,'location-label');
  panel.dispatchEvent(new a.window.PointerEvent('pointerdown',{bubbles:true}));
  assert.equal(panel.hidden,false);
  a.document.dispatchEvent(new a.window.KeyboardEvent('keydown',{key:'Escape',bubbles:true}));
  assert.equal(panel.hidden,true);assert.equal(a.document.activeElement,toggle);
  toggle.click();a.document.querySelector('header').dispatchEvent(new a.window.PointerEvent('pointerdown',{bubbles:true}));
  assert.equal(panel.hidden,true);
  toggle.click();a.document.querySelector('#network-settings-close').click();
  assert.equal(panel.hidden,true);
 }finally{await a.close();}
});

test('LAN helper disables on the loopback range and stays disabled during requests',async()=>{
 const a=await app();
 try{
  const button=a.document.querySelector('#lan-access');
  for(const ip of ['127.0.0.1','127.0.0.2']){
   a.frame({...a.state,Listening:ip+':56700'});
   assert.equal(button.disabled,true);assert.match(button.title,/loopback/);
  }
  a.frame(a.state);assert.equal(button.disabled,false);
  let complete;
  a.window.go.app.App.RequestLANAccess=()=>new Promise(resolve=>{complete=resolve;});
  button.click();assert.equal(button.disabled,true);
  a.frame(a.state);assert.equal(button.disabled,true);
  complete();await tick();assert.equal(button.disabled,false);
 }finally{await a.close();}
});
test('LAN helper errors expire without clearing a newer operation error',async()=>{
 const a=await app();
 try{
  const button=a.document.querySelector('#lan-access');
  a.window.go.app.App.RequestLANAccess=async()=>{throw new Error('No LAN interface available');};
  button.click();await tick();
  assert.match(a.document.querySelector('#error').textContent,/No LAN interface/);
  a.runTimers(6000);assert.equal(a.document.querySelector('#error').textContent,'');
  button.click();await tick();
  a.window.go.app.App.ListenOn=async()=>{throw new Error('Cannot bind interface');};
  const select=a.document.querySelector('#interface');
  select.value='192.168.1.10';select.dispatchEvent(new a.window.Event('change'));await tick();
  a.runTimers(6000);assert.match(a.document.querySelector('#error').textContent,/Cannot bind interface/);
 }finally{await a.close();}
});
test('device card actions use text and labeled icon groups',async()=>{
 const a=await app();
 try{
  const card=a.document.querySelector('article');
  const groups=card.querySelectorAll('.card-actions>.action-group');
  assert.equal(groups.length,1);
  const buttons=[...card.querySelectorAll('.card-actions button')];
  assert.deepEqual(buttons.map(button=>button.getAttribute('aria-label')||button.textContent),['Preview','Edit identity','Disable','Remove']);
  assert.deepEqual(buttons.slice(2).map(button=>button.title),['Disable','Remove']);
  assert.equal(buttons[0].textContent,'Preview');assert.equal(buttons[1].textContent,'Edit identity');
  assert.equal(buttons.slice(2).every(button=>button.querySelector('svg')&&button.textContent.trim()===''),true);
  a.state.Devices[0].Enabled=false;a.frame({...a.state});
  const enable=card.querySelector('.enable');
  assert.equal(enable.getAttribute('aria-label'),'Enable');
  assert.equal(enable.title,'Enable');
  assert.equal(enable.classList.contains('will-enable'),true);
 }finally{await a.close();}
});

test('chain length is shown only for chain-capable matrices',async()=>{
 const a=await app();
 try{
  const product=a.document.querySelector('#product');
  product.value='219';product.dispatchEvent(new a.window.Event('change'));
  assert.ok(a.document.querySelector('#width'));assert.equal(a.document.querySelector('#chains'),null);
  product.value='55';product.dispatchEvent(new a.window.Event('change'));
  assert.equal(a.document.querySelector('#chains').value,'5');
  assert.equal(a.document.querySelector('#chains').readOnly,false);
 }finally{await a.close();}
});

test('virtual room uses a labeled screen icon',async()=>{
 const a=await app();
 try{
  const button=a.document.querySelector('#presentation-all');
  assert.equal(button.textContent.trim(),'Virtual room');
  assert.equal(button.title,'Open virtual room');
  assert.ok(button.querySelector('svg'));
  assert.equal(button.querySelector('span').textContent,'Virtual room');
 }finally{await a.close();}
});
test('hovering a rendered zone shows its HSBK tooltip',async()=>{
 const a=await app();
 try{
  a.state.Devices[0].Colors=[{Hue:120,Saturation:50,Brightness:25,Kelvin:3500}];a.frame({...a.state});
  const canvas=a.document.querySelector('article canvas');
  canvas.getBoundingClientRect=()=>({left:0,top:0,width:200,height:100});
  canvas.dispatchEvent(new a.window.PointerEvent('pointermove',{bubbles:true,clientX:100,clientY:44}));
  const tooltip=a.document.querySelector('#zone-tooltip');
  assert.equal(a.document.querySelector('.info'),null);
  assert.match(a.document.querySelector('.state').title,/Power level 65535 \(100%\)/);
  assert.equal(tooltip.hidden,true);
  a.runTimers(350);
  assert.equal(tooltip.hidden,false);
  assert.match(tooltip.textContent,/Color · H 120\.0° · S 50\.0% · B 25\.0% · K 3500/);
  canvas.dispatchEvent(new a.window.PointerEvent('pointerleave',{bubbles:true}));
  assert.equal(tooltip.hidden,true);
 }finally{await a.close();}
});

test('device badge reflects power without animation flicker',async()=>{
 const a=await app();
 try{
  const badge=a.document.querySelector('article .state');
  assert.equal(badge.textContent,'On');
  a.state.Devices[0].Active=true;a.frame({...a.state});assert.equal(badge.textContent,'On');
  a.state.Devices[0].Active=false;a.frame({...a.state});assert.equal(badge.textContent,'On');
  a.state.Devices[0].Power=0;a.frame({...a.state});assert.equal(badge.textContent,'Off');
  a.state.Devices[0].Enabled=false;a.frame({...a.state});assert.equal(badge.textContent,'Disabled');
 }finally{await a.close();}
});
