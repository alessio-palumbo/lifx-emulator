import {test} from 'node:test';import assert from 'node:assert/strict';import {rgb} from './color.js';
const c={Hue:0,Saturation:100,Brightness:100,Kelvin:3500};
test('HSB primary colors',()=>{assert.deepEqual(rgb(c),[255,0,0]);assert.deepEqual(rgb({...c,Hue:120}),[0,255,0]);assert.deepEqual(rgb({...c,Hue:240}),[0,0,255]);});
test('zero power and brightness are off; low brightness is distinguishable',()=>{assert.deepEqual(rgb(c,0),[0,0,0]);assert.deepEqual(rgb({...c,Brightness:0}),[0,0,0]);assert.ok(rgb({...c,Brightness:1})[0]>20);});
test('Kelvin affects whites and output is bounded',()=>{const warm=rgb({...c,Saturation:0,Kelvin:2500}),cool=rgb({...c,Saturation:0,Kelvin:9000});assert.ok(warm[2]<cool[2]);assert.ok(cool[0]<warm[0]);for(const channel of [...warm,...cool])assert.ok(channel>=0&&channel<=255);});
