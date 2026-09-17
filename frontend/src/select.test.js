import {test} from 'node:test';
import assert from 'node:assert/strict';
import {findMatch, SelectControl} from './select.js';
const options=[{value:'176',textContent:'LIFX Ceiling'},{value:'27',textContent:'LIFX A19'},{value:'55',textContent:'LIFX Tile'},{value:'57',textContent:'LIFX Candle Color'},{value:'5',textContent:'Disabled',disabled:true}];
test('find registry names despite brand prefix and prefer exact PIDs',()=>{
 assert.equal(findMatch(options,'tile'),2);assert.equal(findMatch(options,'CANDLE COLOR'),3);assert.equal(findMatch(options,'27'),1);assert.equal(findMatch(options,'17'),0);assert.equal(findMatch(options,'5'),2);assert.equal(findMatch(options,'missing'),-1);
});
function control(hidden=true){return {select:{options},menu:{hidden,children:[]},active:1,open(){this.menu.hidden=false},close(){this.menu.hidden=true},highlight(index){this.active=index}};}
const key=key=>({key,preventDefault(){}});
test('first arrow key moves immediately from focused closed control',()=>{
 const c=control();SelectControl.prototype.keydown.call(c,key('ArrowDown'));assert.equal(c.menu.hidden,false);assert.equal(c.active,2);
 const home=control();SelectControl.prototype.keydown.call(home,key('Home'));assert.equal(home.active,0);
 const end=control();SelectControl.prototype.keydown.call(end,key('End'));assert.equal(end.active,3);
});
test('typing on a focused closed control locates names or product IDs',()=>{
 for(const [query,index] of [['Tile',2],['57',3]]){const c=control();for(const character of query)SelectControl.prototype.keydown.call(c,key(character));assert.equal(c.menu.hidden,false);assert.equal(c.active,index)}
 const c=control(false);c.search='candle col';c.lastSearch=Date.now();SelectControl.prototype.keydown.call(c,key(' '));assert.equal(c.search,'candle col ');
});
