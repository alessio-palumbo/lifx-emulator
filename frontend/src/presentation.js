const storageKey = 'lifx-emulator.presentation-layout.v1';
const clamp = (value, minimum, maximum) => Math.min(maximum, Math.max(minimum, value));
const toolbarIcons = {
  arrange: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 2v20M2 12h20M12 2 9 5M12 2l3 3M22 12l-3-3M22 12l-3 3M12 22l-3-3M12 22l3-3M2 12l3-3M2 12l3 3"/></svg>',
  fullscreen: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 3H3v5M16 3h5v5M21 16v5h-5M8 21H3v-5"/></svg>',
  restore: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 8h5V3M21 8h-5V3M16 21v-5h5M8 21v-5H3"/></svg>',
  close: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m5 5 14 14M19 5 5 19"/></svg>'
};

function savedLayout(storage) {
  try { return JSON.parse(storage?.getItem(storageKey) || '{}') || {}; } catch { return {}; }
}
function defaultLayout(index) {
  return {x: 4 + (index % 3) * 32, y: 6 + (Math.floor(index / 3) % 3) * 30, width: 28, height: 24};
}

export function createPresentation({root, draw, devices, runtime = {}, storage = globalThis.localStorage}) {
  const overlay = document.createElement('section');
  overlay.id = 'presentation'; overlay.hidden = true;
  overlay.innerHTML = `<div class="presentation-toolbar"><strong id="presentation-title">Presentation</strong><span class="presentation-help"></span><button id="presentation-arrange" class="presentation-icon" type="button" aria-label="Arrange devices" title="Arrange devices" aria-pressed="false">${toolbarIcons.arrange}</button><button id="presentation-fullscreen" class="presentation-icon" type="button" aria-label="Enter full screen" title="Enter full screen">${toolbarIcons.fullscreen}</button><button id="presentation-close" class="presentation-icon" type="button" aria-label="Close presentation" title="Close presentation">${toolbarIcons.close}</button></div><div id="presentation-stage" aria-label="Virtual room canvas"></div>`;
  root.append(overlay);
  const stage=overlay.querySelector('#presentation-stage'), title=overlay.querySelector('#presentation-title'), help=overlay.querySelector('.presentation-help'), arrange=overlay.querySelector('#presentation-arrange'), fullscreen=overlay.querySelector('#presentation-fullscreen');
  let mode='all', focused='', arranging=false, fullscreenActive=false, layout=savedLayout(storage), drag;
  const persist=()=>{try{storage?.setItem(storageKey,JSON.stringify(layout));}catch{}};
  function setArrange(value) {
    arranging=mode==='all'&&value; overlay.classList.toggle('arranging',arranging);
    arrange.setAttribute('aria-pressed',String(arranging)); arrange.setAttribute('aria-label',arranging?'Finish arranging':'Arrange devices'); arrange.title=arranging?'Finish arranging':'Arrange devices';
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
    arrange.hidden=mode==='single';title.textContent=mode==='single'?shown[0]?.Label||'Device preview':'Virtual room';
  }
  function open(nextMode,serial=''){mode=nextMode;focused=serial;overlay.hidden=false;document.body.classList.add('presenting');setArrange(false);render();overlay.querySelector('#presentation-close').focus();}
  async function setFullscreen(value){try{if(value){if(runtime.WindowFullscreen)runtime.WindowFullscreen();else await document.documentElement.requestFullscreen?.();}else{if(runtime.WindowUnfullscreen)runtime.WindowUnfullscreen();else if(document.fullscreenElement)await document.exitFullscreen?.();}fullscreenActive=value;fullscreen.innerHTML=value?toolbarIcons.restore:toolbarIcons.fullscreen;fullscreen.setAttribute('aria-label',value?'Exit full screen':'Enter full screen');fullscreen.title=value?'Exit full screen':'Enter full screen';}catch{}}
  function close(){overlay.hidden=true;document.body.classList.remove('presenting');setArrange(false);if(fullscreenActive)setFullscreen(false);}
  function beginPointer(event,node){if(!arranging||event.button!==0)return;event.preventDefault();node.setPointerCapture?.(event.pointerId);const frame=stage.getBoundingClientRect(),rect=node.getBoundingClientRect();drag={node,serial:node.dataset.serial,resize:event.target.classList.contains('resize-handle'),startX:event.clientX,startY:event.clientY,frame,x:rect.left-frame.left,y:rect.top-frame.top,width:rect.width,height:rect.height};}
  stage.addEventListener('pointermove',event=>{if(!drag)return;const dx=event.clientX-drag.startX,dy=event.clientY-drag.startY;let{x,y,width,height}=drag;if(drag.resize){width=clamp(drag.width+dx,80,drag.frame.width-drag.x);height=clamp(drag.height+dy,60,drag.frame.height-drag.y);}else{x=clamp(drag.x+dx,0,drag.frame.width-drag.width);y=clamp(drag.y+dy,0,drag.frame.height-drag.height);}layout[drag.serial]={x:x/drag.frame.width*100,y:y/drag.frame.height*100,width:width/drag.frame.width*100,height:height/drag.frame.height*100};render();});
  const finishPointer=()=>{if(drag){drag=undefined;persist();}};stage.addEventListener('pointerup',finishPointer);stage.addEventListener('pointercancel',finishPointer);
  arrange.onclick=()=>setArrange(!arranging);fullscreen.onclick=()=>setFullscreen(!fullscreenActive);overlay.querySelector('#presentation-close').onclick=close;
  document.addEventListener('keydown',event=>{if(event.key==='Escape'&&!overlay.hidden){event.preventDefault();close();}});
  document.addEventListener('fullscreenchange',()=>{if(!document.fullscreenElement&&fullscreenActive){fullscreenActive=false;fullscreen.innerHTML=toolbarIcons.fullscreen;fullscreen.setAttribute('aria-label','Enter full screen');fullscreen.title='Enter full screen';}});
  window.addEventListener('resize',()=>render());
  return {openDevice:serial=>open('single',serial),openAll:()=>open('all'),close,render,element:overlay};
}
