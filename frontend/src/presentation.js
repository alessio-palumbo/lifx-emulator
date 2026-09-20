const storageKey = 'lifx-emulator.presentation-layout.v1';
const clamp = (value, minimum, maximum) => Math.min(maximum, Math.max(minimum, value));

function savedLayout(storage) {
  try { return JSON.parse(storage?.getItem(storageKey) || '{}') || {}; } catch { return {}; }
}
function defaultLayout(index) {
  return {x: 4 + (index % 3) * 32, y: 6 + (Math.floor(index / 3) % 3) * 30, width: 28, height: 24};
}

export function createPresentation({root, draw, devices, runtime = {}, storage = globalThis.localStorage}) {
  const overlay = document.createElement('section');
  overlay.id = 'presentation'; overlay.hidden = true;
  overlay.innerHTML = `<div class="presentation-toolbar"><strong id="presentation-title">Presentation</strong><span class="presentation-help"></span><button id="presentation-arrange" type="button" aria-pressed="false">Arrange</button><button id="presentation-fullscreen" type="button">Full screen</button><button id="presentation-close" type="button">Close</button></div><div id="presentation-stage" aria-label="Virtual device presentation canvas"></div>`;
  root.append(overlay);
  const stage=overlay.querySelector('#presentation-stage'), title=overlay.querySelector('#presentation-title'), help=overlay.querySelector('.presentation-help'), arrange=overlay.querySelector('#presentation-arrange'), fullscreen=overlay.querySelector('#presentation-fullscreen');
  let mode='all', focused='', arranging=false, fullscreenActive=false, layout=savedLayout(storage), drag;
  const persist=()=>{try{storage?.setItem(storageKey,JSON.stringify(layout));}catch{}};
  function setArrange(value) {
    arranging=mode==='all'&&value; overlay.classList.toggle('arranging',arranging);
    arrange.setAttribute('aria-pressed',String(arranging)); arrange.textContent=arranging?'Done':'Arrange';
    help.textContent=arranging?'Drag devices or their corner handles to resize.':mode==='all'?'Live layout':'Live device preview';
  }
  function item(device) {
    let node=[...stage.children].find(child=>child.dataset.serial===device.Serial);
    if(!node){node=document.createElement('div');node.className='presented-device';node.dataset.serial=device.Serial;node.innerHTML='<canvas></canvas><span class="presented-label"></span><span class="resize-handle" aria-hidden="true"></span>';stage.append(node);node.addEventListener('pointerdown',event=>beginPointer(event,node));}
    return node;
  }
  function render(current=devices()) {
    if(overlay.hidden)return;
    const shown=mode==='single'?current.filter(device=>device.Serial===focused):current;
    if(mode==='single'&&!shown.length){close();return;}
    for(const node of [...stage.children])if(!shown.some(device=>device.Serial===node.dataset.serial))node.remove();
    shown.forEach((device,index)=>{const node=item(device);node.querySelector('.presented-label').textContent=device.Label;if(mode==='single')node.style.cssText='left:3%;top:3%;width:94%;height:94%';else{const position=layout[device.Serial]||(layout[device.Serial]=defaultLayout(index));node.style.left=`${position.x}%`;node.style.top=`${position.y}%`;node.style.width=`${position.width}%`;node.style.height=`${position.height}%`;}draw(node.querySelector('canvas'),device,{presentation:true});});
    arrange.hidden=mode==='single';title.textContent=mode==='single'?shown[0]?.Label||'Device preview':'Virtual device canvas';
  }
  function open(nextMode,serial=''){mode=nextMode;focused=serial;overlay.hidden=false;document.body.classList.add('presenting');setArrange(false);render();overlay.querySelector('#presentation-close').focus();}
  async function setFullscreen(value){try{if(value){if(runtime.WindowFullscreen)runtime.WindowFullscreen();else await document.documentElement.requestFullscreen?.();}else{if(runtime.WindowUnfullscreen)runtime.WindowUnfullscreen();else if(document.fullscreenElement)await document.exitFullscreen?.();}fullscreenActive=value;fullscreen.textContent=value?'Exit full screen':'Full screen';}catch{}}
  function close(){overlay.hidden=true;document.body.classList.remove('presenting');setArrange(false);if(fullscreenActive)setFullscreen(false);}
  function beginPointer(event,node){if(!arranging||event.button!==0)return;event.preventDefault();node.setPointerCapture?.(event.pointerId);const frame=stage.getBoundingClientRect(),rect=node.getBoundingClientRect();drag={node,serial:node.dataset.serial,resize:event.target.classList.contains('resize-handle'),startX:event.clientX,startY:event.clientY,frame,x:rect.left-frame.left,y:rect.top-frame.top,width:rect.width,height:rect.height};}
  stage.addEventListener('pointermove',event=>{if(!drag)return;const dx=event.clientX-drag.startX,dy=event.clientY-drag.startY;let{x,y,width,height}=drag;if(drag.resize){width=clamp(drag.width+dx,80,drag.frame.width-drag.x);height=clamp(drag.height+dy,60,drag.frame.height-drag.y);}else{x=clamp(drag.x+dx,0,drag.frame.width-drag.width);y=clamp(drag.y+dy,0,drag.frame.height-drag.height);}layout[drag.serial]={x:x/drag.frame.width*100,y:y/drag.frame.height*100,width:width/drag.frame.width*100,height:height/drag.frame.height*100};render();});
  const finishPointer=()=>{if(drag){drag=undefined;persist();}};stage.addEventListener('pointerup',finishPointer);stage.addEventListener('pointercancel',finishPointer);
  arrange.onclick=()=>setArrange(!arranging);fullscreen.onclick=()=>setFullscreen(!fullscreenActive);overlay.querySelector('#presentation-close').onclick=close;
  document.addEventListener('keydown',event=>{if(event.key==='Escape'&&!overlay.hidden){event.preventDefault();close();}});
  document.addEventListener('fullscreenchange',()=>{if(!document.fullscreenElement&&fullscreenActive){fullscreenActive=false;fullscreen.textContent='Full screen';}});
  window.addEventListener('resize',()=>render());
  return {openDevice:serial=>open('single',serial),openAll:()=>open('all'),close,render,element:overlay};
}
