import './style.css';
import appIcon from '../../build/appicon.png';
import {cssColor} from './color.js';
import {enhanceSelects} from './select.js';
const api = () => window.go.app.App;
let view = {Devices:[],Recent:[],Interfaces:[]}, products = [], lastActivityKey = '';
const root = document.querySelector('#app');
root.innerHTML = `<header><div class="brand"><img class="mark" src="${appIcon}" alt=""><div><h1>LIFX Emulator</h1><p>Virtual lights. Real LAN traffic.</p></div></div><span class="badge">LOCAL LAB</span></header>
<section class="network"><div class="network-status"><span id="status"></span></div><p id="membership-summary" class="network-identity"></p><label>Listen on <select id="interface"></select></label><button id="lan-access" class="icon-button" hidden aria-label="Request macOS LAN access" title="macOS permission helper: opens a local-network socket to allow a permission prompt. Sends no packet and cannot confirm or reset permission. If denied, enable access in System Settings."><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 7v5h-5M20 12a8 8 0 1 0-2.3 5.7M20 7l-3-3"/></svg></button><div class="network-settings"><button id="network-settings-toggle" class="icon-button" aria-label="Location and group settings" title="Location and group settings" aria-expanded="false" aria-controls="network-settings-panel"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M18.93 9.13L21.82 10.13L21.82 13.87L18.93 14.87L18.93 14.87L20.27 17.62L17.62 20.27L14.87 18.93L14.87 18.93L13.87 21.82L10.13 21.82L9.13 18.93L9.13 18.93L6.38 20.27L3.73 17.62L5.07 14.87L5.07 14.87L2.18 13.87L2.18 10.13L5.07 9.13L5.07 9.13L3.73 6.38L6.38 3.73L9.13 5.07L9.13 5.07L10.13 2.18L13.87 2.18L14.87 5.07L14.87 5.07L17.62 3.73L20.27 6.38L18.93 9.13Z"/><circle cx="12" cy="12" r="3"/></svg></button><section id="network-settings-panel" class="membership-panel" aria-label="Location and group settings" hidden><div class="settings-heading"><h2>Location &amp; group</h2><button id="network-settings-close" type="button" class="icon-button" aria-label="Close settings">×</button></div><form id="membership"><label>Location<input id="location-label" maxlength="32" required></label><label>Group<input id="group-label" maxlength="32" required></label><details class="membership-ids"><summary>Advanced: shared IDs</summary><label>Location UUID<input id="location-id" spellcheck="false" autocomplete="off" required></label><label>Group UUID<input id="group-id" spellcheck="false" autocomplete="off" required></label><p class="hint">Renaming keeps these IDs. Use matching IDs across emulators only to share a location or group intentionally.</p></details><button type="submit">Save location &amp; group</button></form></section></div></section>
<div id="error" role="alert"></div><p id="network-note" role="status"></p><main><section><div class="section-title"><div><h2>Your virtual lights</h2></div><span id="count"></span></div><div id="devices"></div></section><aside><section class="panel"><h2>Add a light</h2><form id="add"><label>Product<select id="product"></select></label><label>Label<input id="label" value="Virtual light" maxlength="32" required></label><div id="topology"></div><button class="primary">Create virtual light</button></form><p class="hint">Unique locally administered targets are generated automatically. Matrix dimensions describe physical packet layout; previews use the library surface mapping.</p></section><section class="panel activity"><h2>Recent traffic <button id="traffic-mode" type="button" class="traffic-mode" aria-pressed="false" title="Pause live traffic to inspect messages">Pause</button></h2><p class="hint" id="transport"></p><div id="activity" tabindex="0" aria-label="Recent LAN traffic"></div></section></aside></main>`;
const el=id=>document.getElementById(id);
const escape=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
let networkNoteTimer;
function networkNote(message, timeout=0){clearTimeout(networkNoteTimer);el('network-note').textContent=message;if(timeout)networkNoteTimer=setTimeout(()=>{el('network-note').textContent='';},timeout);}
root.addEventListener('click',event=>{const button=event.target.closest('button');if(!button)return;button.classList.add('clicked');setTimeout(()=>button.classList.remove('clicked'),220);},true);
function collapseIdentity(event){for(const form of root.querySelectorAll('.identity:not([hidden])')){if(!form.contains(event.target)&&!form.closest('article').querySelector('.edit').contains(event.target))form.hidden=true;}}
document.addEventListener('pointerdown',collapseIdentity);
document.addEventListener('focusin',collapseIdentity);
let errorTimer;
function error(e,timeout=0){clearTimeout(errorTimer);el('error').textContent=String(e||'');if(timeout)errorTimer=setTimeout(()=>{el('error').textContent='';},timeout);}
async function action(fn){try{await fn();error('');await refresh();}catch(e){error(e);}}
async function refresh(){view=await api().Snapshot();render();}
function topology(){const p=products.find(p=>p.ID===Number(el('product').value));el('topology').innerHTML=p?.Multizone?'<label>Physical zones<input id="zones" type="number" min="1" max="255" value="16"></label>':p?.Matrix?'<div class="dimensions"><label>Send width<input id="width" type="number" min="1" max="255" value="8"></label><label>Height<input id="height" type="number" min="1" max="255" value="8"></label></div><label>Chain length<input id="chains" type="number" min="1" max="16" value="'+(p.Chain?5:1)+'" '+(!p.Chain?'readonly':'')+'></label><label>Orientation<select id="orientation"><option value="0">Right side up</option><option value="1">Upside down</option><option value="2">Face up</option><option value="3">Face down</option><option value="4">Left</option><option value="5">Right</option></select></label>':'';enhanceSelects(root);}
el('product').onchange=topology;
let lanAccessBusy=false;
const lanAccessTitle=el('lan-access').title;
function renderLANAccess(){
 const button=el('lan-access');
 const loopback=/^127\./.test(view.Listening?.split(':')[0]||'');
 button.hidden=view.Platform!=='darwin';
 button.disabled=loopback||lanAccessBusy||interfaceChanging;
 button.title=loopback?'LAN access is unavailable on loopback. Select a LAN interface or 0.0.0.0.':lanAccessTitle;
}
el('lan-access').onclick=async()=>{const button=el('lan-access');if(button.disabled)return;lanAccessBusy=true;renderLANAccess();networkNote('Allow local-network access if macOS prompts. If access was previously denied, enable lifx-emulator in System Settings → Privacy & Security → Local Network.');try{await api().RequestLANAccess();error('');networkNote('LAN access request completed. Allow any macOS prompt, then retry discovery in your LIFX LAN client.',6000);}catch(e){networkNote('');error(e,6000);}finally{lanAccessBusy=false;renderLANAccess();}};
let interfaceChanging=false;
el('interface').onchange=async()=>{
 const select=el('interface');
 if(interfaceChanging||select.value===view.Listening?.split(':')[0])return;
 interfaceChanging=true;select.disabled=true;renderLANAccess();enhanceSelects(root);
 try{await api().ListenOn(select.value);error('');}catch(e){error(e);}
 finally{try{await refresh();}catch(e){error(e);}interfaceChanging=false;select.disabled=false;render();}
};
el('add').onsubmit=e=>{e.preventDefault();const n=(id,fallback=0)=>Number(el(id)?.value??fallback);action(()=>api().Add({Serial:'',Label:el('label').value,Product:n('product'),Enabled:true,Zones:n('zones'),Width:n('width'),Height:n('height'),Chains:n('chains'),Orientations:Array(n('chains')).fill(n('orientation'))}));};
function setNetworkSettings(open,restoreFocus=false){
 el('network-settings-panel').hidden=!open;
 el('network-settings-toggle').setAttribute('aria-expanded',String(open));
 if(open)el('location-label').focus();else if(restoreFocus)el('network-settings-toggle').focus();
}
el('network-settings-toggle').onclick=()=>setNetworkSettings(el('network-settings-panel').hidden);
el('network-settings-close').onclick=()=>setNetworkSettings(false,true);
for(const eventName of ['pointerdown','focusin'])document.addEventListener(eventName,event=>{if(!el('network-settings-panel').hidden&&!event.target.closest('.network-settings'))setNetworkSettings(false);});
document.addEventListener('keydown',event=>{if(event.key==='Escape'&&!el('network-settings-panel').hidden){event.preventDefault();setNetworkSettings(false,true);}});
let membershipDirty=false;
el('membership').addEventListener('input',()=>{membershipDirty=true;});
el('membership').onsubmit=event=>{event.preventDefault();action(async()=>{await api().UpdateMembership(el('location-label').value,el('location-id').value,el('group-label').value,el('group-id').value);membershipDirty=false;setNetworkSettings(false,true);});};
function renderMembership(){
 el('membership-summary').textContent=[view.Location?.Label&&`Location: ${view.Location.Label}`,view.Group?.Label&&`Group: ${view.Group.Label}`].filter(Boolean).join(' · ');
 if(membershipDirty)return;
 for(const [prefix,entry] of [['location',view.Location],['group',view.Group]]){if(entry){el(prefix+'-label').value=entry.Label;el(prefix+'-id').value=entry.ID;}}
}
function render(){
 renderMembership();
 renderLANAccess();
 const t=view.Transport||{}; const transportLines=`<span>RX ${t.Received||0} · decoded ${t.Decoded||0} · TX ${t.Replies||0}</span><span>Dropped ${(t.Filtered||0)+(t.Invalid||0)} · send errors ${t.SendErrors||0}</span>`;if(el('transport').innerHTML!==transportLines)el('transport').innerHTML=transportLines; el('transport').title=[t.LastPeer&&`Last sender: ${t.LastPeer}`,t.LastError].filter(Boolean).join('\n');
 el('status').textContent=view.Error?`● Listener stopped: ${view.Error}`:`● Listening · ${view.Listening}`;
 el('count').textContent=`${view.Devices.length} lights`;
 if(el('interface').options.length===0){el('interface').innerHTML=view.Interfaces.map(ip=>`<option>${escape(ip)}</option>`).join('');}
 if(!interfaceChanging)el('interface').value=view.Listening?.split(':')[0]||'0.0.0.0';
 const container=el('devices');
 for(const existing of [...container.children]){if(!view.Devices.some(d=>d.Serial===existing.dataset.serial))existing.remove();}
 for(const d of view.Devices){let card=[...container.children].find(c=>c.dataset.serial===d.Serial);if(!card){card=document.createElement('article');card.dataset.serial=d.Serial;card.innerHTML=`<div class="card-title"><div><h3></h3><p></p></div><span class="state"></span></div><div class="preview"><canvas aria-label="Live device color preview"></canvas></div><div class="metadata"></div><div class="actions"><button class="enable"></button><button class="edit">Edit identity</button><button class="remove">Remove</button></div><form class="identity" hidden><label>Label<input class="edit-label" maxlength="32" required></label><label>Target serial<input class="edit-serial" pattern="[0-9a-fA-F]{12}" required></label><p class="hint">Disable before changing the target serial.</p><div class="actions"><button>Save</button><button type="button" class="cancel">Cancel</button></div></form>`;container.append(card);
 card.querySelector('.enable').onclick=()=>{const current=view.Devices.find(v=>v.Serial===card.dataset.serial);action(()=>api().Update(current.Serial,current.Serial,current.Label,!current.Enabled));};
 card.querySelector('.remove').onclick=()=>action(()=>api().Remove(card.dataset.serial));
 card.querySelector('.edit').onclick=()=>{const form=card.querySelector('.identity');if(!form.hidden){form.hidden=true;return;}for(const other of container.querySelectorAll('.identity'))other.hidden=true;form.hidden=false;const current=view.Devices.find(v=>v.Serial===card.dataset.serial);card.querySelector('.edit-label').value=current.Label;card.querySelector('.edit-serial').value=current.Serial;card.querySelector('.edit-serial').readOnly=view.Devices.find(v=>v.Serial===card.dataset.serial).Enabled;};
 card.querySelector('.cancel').onclick=()=>{card.querySelector('.identity').hidden=true;};
 card.querySelector('.identity').onsubmit=e=>{e.preventDefault();action(async()=>{await api().Update(card.dataset.serial,card.querySelector('.edit-serial').value,card.querySelector('.edit-label').value,view.Devices.find(v=>v.Serial===card.dataset.serial).Enabled);card.querySelector('.identity').hidden=true;});};
 }
 card.querySelector('h3').textContent=d.Label;card.querySelector('.card-title p').textContent=d.Model;card.querySelector('.state').textContent=!d.Enabled?'Disabled':d.Active?'Animating':d.Power?'On':'Off';card.classList.toggle('disabled',!d.Enabled);card.querySelector('.metadata').textContent=`${d.Serial} · PID ${d.Product} · ${d.Kind.replaceAll('_',' ')} · ${Math.round(d.Power/65535*100)}% power`;card.querySelector('.enable').textContent=d.Enabled?'Disable':'Enable';draw(card.querySelector('canvas'),d);
 }
 if(!view.Devices.length)container.innerHTML='<p class="empty">Add a virtual light to start your LAN lab.</p>';else container.querySelector('.empty')?.remove();
 renderTraffic();
 enhanceSelects(root);
}
let trafficPaused = false;
function setTrafficPaused(paused){
 trafficPaused=paused;
 const button=el('traffic-mode');
 button.textContent=paused?'Resume live':'Pause';
 button.setAttribute('aria-pressed',String(paused));
 button.title=paused?'Displayed history is frozen. Resume to show the latest traffic.':'Pause live traffic to inspect messages';
 if(!paused){el('activity').scrollTop=0;lastActivityKey='';renderTraffic();}
}
el('traffic-mode').onclick=()=>setTrafficPaused(!trafficPaused);
el('activity').addEventListener('scroll',()=>{if(el('activity').scrollTop>0&&!trafficPaused)setTrafficPaused(true);});
function renderTraffic(){
 if(trafficPaused)return;
 const activityKey=(view.Recent?.at(-1)?.At||'')+'|'+(view.Recent?.length||0);if(activityKey!==lastActivityKey||!el('activity').innerHTML){lastActivityKey=activityKey;const activity=el('activity');activity.innerHTML=(view.Recent||[]).slice().reverse().map(a=>`<div data-key="${escape([a.At,a.Direction,a.Target,a.Type,a.Sequence].join('|'))}"><time>${new Date(a.At).toLocaleTimeString()}</time><b class="traffic-direction">${escape(a.Direction||'RX')}</b><code>${escape(a.Target.slice(-6))}</code><span class="message-type" title="${escape(a.Label||view.Devices.find(d=>d.Serial===a.Target)?.Label||a.Target)} · ${escape(a.Target)} · ${escape(a.TypeName||String(a.Type))} · type ${a.Type} · ${escape(a.Peer||'')} · source ${a.Source||0} · seq ${a.Sequence||0} · ${a.Direction==='TX'?(a.Error?'write failed: '+escape(a.Error):'UDP write succeeded; delivery unconfirmed'):(a.Replies||0)+' replies'+(a.Error?' · '+escape(a.Error):'')}">${escape(a.TypeName||String(a.Type))}${a.Error?' ⚠':a.Direction==='RX'&&!a.Replies?' · no reply':''}</span></div>`).join('')||'<p class="hint">Waiting for a LAN client…</p>';}
}
function draw(canvas,d){
 const s=d.Surface;const width=Math.max(200,canvas.parentElement.clientWidth),height=d.Kind==='matrix'?160:100;const ratio=window.devicePixelRatio||1;if(canvas.width!==Math.round(width*ratio)||canvas.height!==height*ratio){canvas.width=Math.round(width*ratio);canvas.height=height*ratio;}canvas.style.width=width+'px';canvas.style.height=height+'px';const ctx=canvas.getContext('2d');ctx.setTransform(ratio,0,0,ratio,0,0);ctx.clearRect(0,0,width,height);
 if(d.Kind==='single_zone'){ctx.fillStyle=cssColor(d.Colors[0],d.Power);ctx.shadowColor=ctx.fillStyle;ctx.shadowBlur=28;ctx.beginPath();ctx.arc(width/2,44,32,0,Math.PI*2);ctx.fill();ctx.shadowBlur=0;ctx.fillStyle='#68707f';ctx.fillRect(width/2-14,77,28,8);return;}
 const cells=[];
 if(d.Kind==='matrix'){for(const chain of s.Matrix?.Chains||[]){for(let y=0;y<chain.Rows.length;y++){const row=chain.Rows[y];for(let x=0;x<row.Cols;x++){if(!row.HiddenCols?.includes(x))cells.push([chain.Bounds.X+row.Offset+x,chain.Bounds.Y+y,s.Matrix.Chains.indexOf(chain)]);}}}}else{for(let x=0;x<s.Width;x++)cells.push([x,0]);}
 const chainGap=d.Kind==='matrix'?4:0;const gaps=chainGap*Math.max(0,(s.Matrix?.Chains.length||1)-1);
 const step=Math.min((width-32-gaps)/s.Width,(height-24)/s.Height,d.Kind==='matrix'?18:30);const offsetX=(width-step*s.Width-gaps)/2,offsetY=(height-step*s.Height)/2;
 for(const [x,y,chain=0]of cells){ctx.fillStyle=cssColor(d.Colors[y*s.Width+x],d.Power);ctx.beginPath();ctx.roundRect(offsetX+x*step+chain*chainGap,offsetY+y*step,Math.max(1,step-2),Math.max(1,step-2),Math.min(3,step/4));ctx.fill();}
}
async function boot(){try{products=await api().Products();el('product').innerHTML=products.map(p=>`<option value="${p.ID}">${escape(p.Name)} · ${p.ID}</option>`).join('');el('product').value='27';topology();await refresh();window.runtime.EventsOn('frame',next=>{view={...view,...next};render();});window.addEventListener('resize',render);}catch(e){error(`Desktop bridge unavailable: ${e}`);}}
boot();
