import {test} from 'node:test';
import assert from 'node:assert/strict';
import {Window} from 'happy-dom';
import {createPresentation} from './presentation.js';

function setup(){
 const window=new Window();globalThis.window=window;globalThis.document=window.document;
 const root=document.createElement('div');document.body.append(root);
 const current=[{Serial:'one',Label:'Bulb'},{Serial:'two',Label:'Tiles'}];
 const stored=new Map(),storage={getItem:key=>stored.get(key),setItem:(key,value)=>stored.set(key,value)};
 let draws=0,fullscreen=0,unfullscreen=0;
 const presentation=createPresentation({root,devices:()=>current,draw:()=>draws++,storage,runtime:{WindowFullscreen:()=>fullscreen++,WindowUnfullscreen:()=>unfullscreen++}});
 return {window,document,root,current,stored,presentation,counts:()=>({draws,fullscreen,unfullscreen}),close:()=>window.happyDOM.abort()};
}

test('presents every device, supports focused preview, and closes with Escape',async()=>{
 const a=setup();try{
  a.presentation.openAll();
  assert.equal(a.presentation.element.hidden,false);
  assert.equal(a.document.body.classList.contains('presenting'),true);
  assert.equal(a.presentation.element.querySelectorAll('.presented-device').length,2);
  assert.ok(a.counts().draws>=2);
  a.presentation.openDevice('two');
  assert.equal(a.presentation.element.querySelectorAll('.presented-device').length,1);
  assert.equal(a.presentation.element.querySelector('.presented-label').textContent,'Tiles');
  assert.equal(a.presentation.element.querySelector('#presentation-arrange').hidden,true);
  a.document.dispatchEvent(new a.window.KeyboardEvent('keydown',{key:'Escape',bubbles:true}));
  assert.equal(a.presentation.element.hidden,true);
  assert.equal(a.document.body.classList.contains('presenting'),false);
 }finally{await a.close();delete globalThis.window;delete globalThis.document;}
});

test('arrangement drag persists normalized layout and fullscreen uses runtime',async()=>{
 const a=setup();try{
  a.presentation.openAll();const overlay=a.presentation.element,stage=overlay.querySelector('#presentation-stage'),node=overlay.querySelector('[data-serial="one"]');
  stage.getBoundingClientRect=()=>({left:0,top:0,width:1000,height:500});
  node.getBoundingClientRect=()=>({left:40,top:30,width:280,height:120});
  overlay.querySelector('#presentation-arrange').click();
  assert.equal(overlay.classList.contains('arranging'),true);
  node.dispatchEvent(new a.window.PointerEvent('pointerdown',{bubbles:true,button:0,clientX:50,clientY:40,pointerId:1}));
  stage.dispatchEvent(new a.window.PointerEvent('pointermove',{bubbles:true,clientX:150,clientY:90,pointerId:1}));
  stage.dispatchEvent(new a.window.PointerEvent('pointerup',{bubbles:true,pointerId:1}));
  let saved=JSON.parse(a.stored.values().next().value);
  assert.equal(Math.round(saved.one.x),14);assert.equal(Math.round(saved.one.y),16);
  const handle=node.querySelector('.resize-handle');
  handle.dispatchEvent(new a.window.PointerEvent('pointerdown',{bubbles:true,button:0,clientX:310,clientY:140,pointerId:2}));
  stage.dispatchEvent(new a.window.PointerEvent('pointermove',{bubbles:true,clientX:430,clientY:220,pointerId:2}));
  stage.dispatchEvent(new a.window.PointerEvent('pointerup',{bubbles:true,pointerId:2}));
  saved=JSON.parse(a.stored.values().next().value);
  assert.equal(Math.round(saved.one.width),40);assert.equal(Math.round(saved.one.height),40);
  overlay.querySelector('#presentation-fullscreen').click();await Promise.resolve();
  assert.equal(a.counts().fullscreen,1);
  overlay.querySelector('#presentation-close').click();await Promise.resolve();
  assert.equal(a.counts().unfullscreen,1);
 }finally{await a.close();delete globalThis.window;delete globalThis.document;}
});
