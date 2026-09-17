import {test, after} from 'node:test';
import assert from 'node:assert/strict';
import {Window} from 'happy-dom';

const window = new Window();
globalThis.document = window.document;
globalThis.MutationObserver = window.MutationObserver;
globalThis.Event = window.Event;
globalThis.window = window;
const {findMatch, matchingIndices, SelectControl} = await import('./select.js');
after(() => window.happyDOM.abort());
const options = [
  {value:'176',textContent:'LIFX Ceiling · 176'},
  {value:'27',textContent:'LIFX A19 · 27'},
  {value:'55',textContent:'LIFX Tile · 55'},
  {value:'57',textContent:'LIFX Candle Color · 57'},
  {value:'5',textContent:'Disabled',disabled:true},
  {value:'550',textContent:'LIFX Tile · 550'},
];
function control(id='product') {
  document.body.innerHTML = '<label>Product<select id="product"></select></label><button id="outside">Outside</button>';
  const select = document.querySelector('select');
  select.id = id;
  for (const option of options) {
    const item = document.createElement('option');
    Object.assign(item, option);
    select.append(item);
  }
  select.selectedIndex = 1;
  return new SelectControl(select);
}
function key(element, key) {
  element.dispatchEvent(new window.KeyboardEvent('keydown', {key, bubbles:true, cancelable:true}));
}
function query(c, value) {
  c.searchInput.value = value;
  c.searchInput.dispatchEvent(new window.Event('input', {bubbles:true}));
}
test('name and PID matches are filtered, with exact PIDs first', () => {
  assert.equal(findMatch(options,'tile'),2);
  assert.equal(findMatch(options,'CANDLE COLOR'),3);
  assert.equal(findMatch(options,'27'),1);
  assert.equal(findMatch(options,'17'),0);
  assert.equal(findMatch(options,'5'),2);
  assert.equal(findMatch(options,'missing'),-1);
  assert.deepEqual(matchingIndices(options,'55'),[2,5]);
  assert.deepEqual(matchingIndices(options,'tile 550'),[5]);
});
test('first arrow key navigates a focused closed control', () => {
  const c = control();
  c.button.focus();
  key(c.button,'ArrowDown');
  assert.equal(c.menu.hidden,false);
  assert.equal(c.active,2);
  key(c.button,'End');
  assert.equal(c.active,5);
  key(c.button,'Home');
  assert.equal(c.active,0);
});
test('click opens visible search; typing filters and selects the first match', () => {
  const c = control();
  c.button.click();
  assert.equal(document.activeElement,c.searchInput);
  assert.equal(c.searchInput.parentElement,c.wrapper);
  assert.equal(c.menu.contains(c.searchInput),false);
  assert.equal(c.wrapper.classList.contains('searching'),true);
  query(c,'candle color');
  assert.equal(c.searchInput.value,'candle color');
  assert.equal(c.list.children.length,1);
  assert.match(c.list.firstChild.textContent,/Candle Color/);
  assert.equal(c.result.textContent,'1 match');
  key(c.searchInput,'Enter');
  assert.equal(c.select.value,'57');
  assert.equal(c.menu.hidden,true);
  assert.equal(c.searchInput.hidden,true);
  assert.equal(c.wrapper.classList.contains('searching'),false);
  assert.match(c.button.textContent,/Candle Color/);
});
test('typing on the closed control starts a visible PID search', () => {
  const c = control();
  key(c.button,'5');
  assert.equal(c.searchInput.value,'5');
  assert.equal(document.activeElement,c.searchInput);
  query(c,'55');
  assert.equal(c.active,2);
  key(c.searchInput,'ArrowDown');
  assert.equal(c.active,5);
  key(c.searchInput,'Enter');
  assert.equal(c.select.value,'550');
});
test('no matches cannot select a hidden option; clearing restores options', () => {
  const c = control();
  c.open(true);
  query(c,'missing');
  assert.equal(c.result.textContent,'0 matches');
  key(c.searchInput,'Enter');
  assert.equal(c.select.value,'27');
  query(c,'');
  assert.equal(c.list.children.length,5);
  query(c,'tile');
  key(c.searchInput,'Escape');
  assert.equal(c.menu.hidden,true);
  assert.equal(c.searchInput.value,'');
  assert.equal(document.activeElement,c.button);
});
test('wheel momentum is consumed by the dropdown, including at its edge', () => {
  const c = control();
  c.open();
  const event = new window.WheelEvent('wheel',{deltaY:1200,bubbles:true,cancelable:true});
  c.list.dispatchEvent(event);
  assert.equal(event.defaultPrevented,true);
  assert.equal(c.list.scrollTop,1200);
});
test('navigation scrolls the option list without scrolling the page', () => {
  const c = control();
  const last = c.list.lastChild;
  Object.defineProperty(c.list,'clientHeight',{value:100});
  Object.defineProperty(last,'offsetTop',{value:300});
  Object.defineProperty(last,'offsetHeight',{value:30});
  last.scrollIntoView = () => assert.fail('must not scroll the page');
  c.highlight(5);
  assert.equal(c.list.scrollTop,230);
});
test('outside pointer interaction dismisses an open dropdown', () => {
  const c = control();
  c.open(true);
  document.querySelector('#outside').dispatchEvent(new window.PointerEvent('pointerdown',{bubbles:true}));
  assert.equal(c.menu.hidden,true);
});

test('interface and orientation selectors have no search field and keep keyboard navigation', () => {
  for(const id of ['interface','orientation']){
    const c = control(id);
    c.button.click();
    assert.equal(c.wrapper.querySelector('input'),null);
    assert.equal(c.result.hidden,true);
    assert.equal(document.activeElement,c.button);
    key(c.button,'ArrowDown');
    key(c.button,'Enter');
    assert.equal(c.select.value,'55');
    assert.equal(c.menu.hidden,true);
  }
});
