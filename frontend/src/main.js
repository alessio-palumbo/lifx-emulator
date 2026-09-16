import './style.css';
import {cssColor} from './color.js';
const api = () => window.go.app.App;
let view = {Devices:[],Recent:[],Interfaces:[]}, products = [], editing = new Set(), lastActivityKey = '';
const root = document.querySelector('#app');
root.innerHTML = `<header><div class="brand"><span class="mark">✳</span><div><h1>LIFX Emulator</h1><p>Virtual lights. Real LAN traffic.</p></div></div><span class="badge">LOCAL LAB</span></header>
<section class="network"><span id="status"></span><label>Listen on <select id="interface"></select></label><button id="rebind">Apply interface</button></section>
<div id="error" role="alert"></div><main><section><div class="section-title"><div><h2>Your virtual lights</h2><p>State and animations are evaluated in Go.</p></div><span id="count"></span></div><div id="devices"></div></section><aside><section class="panel"><h2>Add a light</h2><form id="add"><label>Product<select id="product"></select></label><label>Label<input id="label" value="Virtual light" maxlength="32" required></label><div id="topology"></div><button class="primary">Create virtual light</button></form><p class="hint">Unique locally administered targets are generated automatically. Matrix dimensions describe physical packet layout; previews use the library surface mapping.</p></section><section class="panel activity"><h2>Recent traffic <span>UDP</span></h2><div id="activity"></div></section></aside></main><footer>UDP 56700 · No accounts · No cloud · Firmware effects excluded</footer>`;
const el=id=>document.getElementById(id);
const escape=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
function error(e){el('error').textContent=String(e||'');}
async function action(fn){try{await fn();error('');await refresh();}catch(e){error(e);}}
async function refresh(){view=await api().Snapshot();render();}
function topology(){const p=products.find(p=>p.ID===Number(el('product').value));el('topology').innerHTML=p?.Multizone?'<label>Physical zones<input id="zones" type="number" min="1" max="255" value="16"></label>':p?.Matrix?'<div class="dimensions"><label>Send width<input id="width" type="number" min="1" max="255" value="8"></label><label>Height<input id="height" type="number" min="1" max="255" value="8"></label></div><label>Chain length<input id="chains" type="number" min="1" max="16" value="'+(p.Chain?5:1)+'" '+(!p.Chain?'readonly':'')+'></label><label>Orientation<select id="orientation"><option value="0">Right side up</option><option value="1">Upside down</option><option value="2">Face up</option><option value="3">Face down</option><option value="4">Left</option><option value="5">Right</option></select></label>':'';}
el('product').onchange=topology;
el('rebind').onclick=()=>action(()=>api().ListenOn(el('interface').value));
el('add').onsubmit=e=>{e.preventDefault();const n=(id,fallback=0)=>Number(el(id)?.value??fallback);action(()=>api().Add({Serial:'',Label:el('label').value,Product:n('product'),Enabled:true,Zones:n('zones'),Width:n('width'),Height:n('height'),Chains:n('chains'),Orientations:Array(n('chains')).fill(n('orientation'))}));};
function render(){
 el('status').textContent=view.Error?`● Listener stopped: ${view.Error}`:`● Listening · ${view.Listening}`;
 el('count').textContent=`${view.Devices.length} lights`;
 if(el('interface').options.length===0){el('interface').innerHTML=view.Interfaces.map(ip=>`<option>${escape(ip)}</option>`).join('');el('interface').value=view.Listening?.split(':')[0]||'0.0.0.0';}
 const container=el('devices');
 for(const existing of [...container.children]){if(!view.Devices.some(d=>d.Serial===existing.dataset.serial))existing.remove();}
 for(const d of view.Devices){let card=[...container.children].find(c=>c.dataset.serial===d.Serial);if(!card){card=document.createElement('article');card.dataset.serial=d.Serial;card.innerHTML=`<div class="card-title"><div><h3></h3><p></p></div><span class="state"></span></div><div class="preview"><canvas aria-label="Live device color preview"></canvas></div><div class="metadata"></div><div class="actions"><button class="enable"></button><button class="edit">Edit identity</button><button class="remove">Remove</button></div><form class="identity" hidden><label>Label<input class="edit-label" maxlength="32" required></label><label>Target serial<input class="edit-serial" pattern="[0-9a-fA-F]{12}" required></label><p class="hint">Disable before changing the target serial.</p><button>Save</button><button type="button" class="cancel">Cancel</button></form>`;container.append(card);
 card.querySelector('.enable').onclick=()=>{const current=view.Devices.find(v=>v.Serial===card.dataset.serial);action(()=>api().Update(current.Serial,current.Serial,current.Label,!current.Enabled));};
 card.querySelector('.remove').onclick=()=>action(()=>api().Remove(card.dataset.serial));
 card.querySelector('.edit').onclick=()=>{editing.add(card.dataset.serial);card.querySelector('.identity').hidden=false;const current=view.Devices.find(v=>v.Serial===card.dataset.serial);card.querySelector('.edit-label').value=current.Label;card.querySelector('.edit-serial').value=current.Serial;card.querySelector('.edit-serial').readOnly=view.Devices.find(v=>v.Serial===card.dataset.serial).Enabled;};
 card.querySelector('.cancel').onclick=()=>{editing.delete(card.dataset.serial);card.querySelector('.identity').hidden=true;};
 card.querySelector('.identity').onsubmit=e=>{e.preventDefault();action(async()=>{await api().Update(card.dataset.serial,card.querySelector('.edit-serial').value,card.querySelector('.edit-label').value,view.Devices.find(v=>v.Serial===card.dataset.serial).Enabled);editing.delete(card.dataset.serial);card.querySelector('.identity').hidden=true;});};
 }
 card.querySelector('h3').textContent=d.Label;card.querySelector('.card-title p').textContent=d.Model;card.querySelector('.state').textContent=!d.Enabled?'Disabled':d.Active?'Animating':d.Power?'On':'Off';card.classList.toggle('disabled',!d.Enabled);card.querySelector('.metadata').textContent=`${d.Serial} · PID ${d.Product} · ${d.Kind.replaceAll('_',' ')} · ${Math.round(d.Power/65535*100)}% power`;card.querySelector('.enable').textContent=d.Enabled?'Disable':'Enable';draw(card.querySelector('canvas'),d);
 }
 if(!view.Devices.length)container.innerHTML='<p class="empty">Add a virtual light to start your LAN lab.</p>';else container.querySelector('.empty')?.remove();
 const activityKey=(view.Recent?.at(-1)?.At||'')+'|'+(view.Recent?.length||0);if(activityKey!==lastActivityKey||!el('activity').innerHTML){lastActivityKey=activityKey;el('activity').innerHTML=(view.Recent||[]).slice(-18).reverse().map(a=>`<div><time>${new Date(a.At).toLocaleTimeString()}</time><code>${escape(a.Target.slice(-6))}</code><span>${a.Applied?'SET':'QUERY'} ${a.Type}</span></div>`).join('')||'<p class="hint">Waiting for a LAN client…</p>';} 
}
function draw(canvas,d){
 const s=d.Surface;const width=Math.max(200,canvas.parentElement.clientWidth),height=d.Kind==='matrix'?160:100;const ratio=window.devicePixelRatio||1;if(canvas.width!==Math.round(width*ratio)||canvas.height!==height*ratio){canvas.width=Math.round(width*ratio);canvas.height=height*ratio;}canvas.style.width=width+'px';canvas.style.height=height+'px';const ctx=canvas.getContext('2d');ctx.setTransform(ratio,0,0,ratio,0,0);ctx.clearRect(0,0,width,height);
 if(d.Kind==='single_zone'){ctx.fillStyle=cssColor(d.Colors[0],d.Power);ctx.shadowColor=ctx.fillStyle;ctx.shadowBlur=28;ctx.beginPath();ctx.arc(width/2,44,32,0,Math.PI*2);ctx.fill();ctx.shadowBlur=0;ctx.fillStyle='#68707f';ctx.fillRect(width/2-14,77,28,8);return;}
 const cells=[];
 if(d.Kind==='matrix'){for(const chain of s.Matrix?.Chains||[]){for(let y=0;y<chain.Rows.length;y++){const row=chain.Rows[y];for(let x=0;x<row.Cols;x++){if(!row.HiddenCols?.includes(x))cells.push([chain.Bounds.X+row.Offset+x,chain.Bounds.Y+y]);}}}}else{for(let x=0;x<s.Width;x++)cells.push([x,0]);}
 const step=Math.min((width-32)/s.Width,(height-24)/s.Height,d.Kind==='matrix'?18:30);const offsetX=(width-step*s.Width)/2,offsetY=(height-step*s.Height)/2;
 for(const [x,y]of cells){ctx.fillStyle=cssColor(d.Colors[y*s.Width+x],d.Power);ctx.beginPath();ctx.roundRect(offsetX+x*step,offsetY+y*step,Math.max(1,step-2),Math.max(1,step-2),Math.min(3,step/4));ctx.fill();}
}
async function boot(){try{products=await api().Products();el('product').innerHTML=products.map(p=>`<option value="${p.ID}">${escape(p.Name)} · ${p.ID}</option>`).join('');el('product').value='27';topology();await refresh();window.runtime.EventsOn('frame',next=>{view={...view,...next};render();});window.addEventListener('resize',render);}catch(e){error(`Desktop bridge unavailable: ${e}`);}}
boot();
