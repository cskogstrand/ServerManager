// Run from the repository root: node docs/pitlane-concept/check.cjs
// Uses the project's existing jsdom dependency. No new packages required.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { JSDOM } = require('../../webapp/node_modules/jsdom');
const dir = __dirname;
const dom = new JSDOM(fs.readFileSync(path.join(dir,'index.html'),'utf8'),{
  url:'http://localhost/#today', runScripts:'outside-only', pretendToBeVisual:true,
});
const {window:w}=dom;
w.scrollTo=()=>{};
const intervals=new Map();let intervalId=0;
w.setInterval=callback=>{intervals.set(++intervalId,callback);return intervalId;};
w.clearInterval=id=>intervals.delete(id);
w.HTMLDialogElement.prototype.showModal=function(){this.open=true;};
w.HTMLDialogElement.prototype.close=function(){this.open=false;};
w.eval(['map-data.js','watch.js','app.js'].map(file=>fs.readFileSync(path.join(dir,file),'utf8')).join('\n')+'\nwindow.__read = expression => eval(expression);');
const doc=w.document;
const read=expression=>w.__read(expression);
const checkText=text=>assert.ok(doc.querySelector('main').textContent.includes(text),text);
const route=hash=>{w.location.hash=hash;w.dispatchEvent(new w.HashChangeEvent('hashchange'));};
const click=(action,value)=>{
  const selector=`[data-action="${action}"]${value!==undefined?`[data-value="${value}"]`:''}`;
  const element=doc.querySelector(selector);
  assert.ok(element,selector);
  assert.equal(element.disabled,false,selector+' must be enabled');
  element.click();
};
const submit=type=>doc.querySelector(`[data-form="${type}"]`).dispatchEvent(new w.Event('submit',{bubbles:true,cancelable:true}));

// Core promise: choose an experience, get valid defaults, start that exact session.
checkText('Tonight, we drive');
route('new');click('experience','Drift');route('build');
checkText('Ready when you are');
assert.equal(read('ready().ok'),true);
assert.equal(read('ready().server'),'Club 02');
click('edit','cars');
doc.querySelector('[name="grid"]').value='24';submit('cars');
assert.equal(doc.querySelector('[data-action="launch"]').disabled,true);
checkText('has 16 spaces');
click('edit','cars');doc.querySelector('[name="grid"]').value='12';submit('cars');
click('edit','format');doc.querySelector('[name="duration"]').value='120';submit('format');
assert.equal(read('ready().ok'),false,'overlap must prevent start');
click('when','later');assert.equal(read('ready().ok'),true,'a later free time is usable');
click('when','now');click('edit','format');doc.querySelector('[name="duration"]').value='60';submit('format');
click('launch');route('live/session-1');
checkText('An evening, sideways');checkText('Waiting for the first driver');
assert.equal(read("findSession('club').status"),'live','existing session is undisturbed');
assert.equal(read("sessionDrivers(findSession('session-1')).length"),0,'new sessions cannot borrow live drivers');

// A driver handover changes future attribution, and a recap snapshots its own results.
route('live/club');click('handover','club');click('assign','alex');
assert.equal(read('state.seat'),'alex');
assert.equal(read("person('nora').live"),false);
assert.equal(read("person('alex').session"),'club');
click('end','club');click('confirm-end','club');route('recap/club');
checkText('That was a good one');
assert.equal(read("findSession('club').results.length"),6);
assert.equal(read("sessionDrivers(findSession('club')).length"),6,'recap retains results after disconnection');
assert.equal(read("findSession('session-1').status"),'live','ending one session leaves the other running');

// Scheduling, search, duplicate-name validation, and first-use entry are functional.
route('new');click('experience','Practice');route('build');click('when','later');click('launch');route('today');
assert.equal(read("state.sessions.find(s=>s.id==='session-2').status"),'planned');
route('drivers');click('add-driver');doc.querySelector('[name="name"]').value='Nora Hansen';submit('add-driver');
assert.ok(doc.querySelector('.error').textContent.includes('already in your club'));
doc.querySelector('[name="name"]').value='New Driver';submit('add-driver');
checkText('New Driver');
const input=doc.querySelector('[data-search="drivers"]');input.value='zzzz';input.dispatchEvent(new w.Event('input',{bubbles:true}));
checkText('No matching drivers');
route('welcome');submit('connect');assert.ok(doc.querySelector('#dialog-title').textContent.includes('Ready'));
click('start-first');route('new');assert.equal(read('state.sessions.length'),0);
click('experience','Race');route('build');click('launch');route('live/session-1');checkText('Practice');
click('phase','session-1');click('confirm-phase','session-1');checkText('Qualifying');
// Watching is useful even when a driver or a camera is disconnected.
click('reset');route('streams');
const wc=(action,value)=>{
  const selector=`[data-watch-action="${action}"]${value!==undefined?`[data-value="${value}"]`:''}`;
  const el=doc.querySelector(selector);assert.ok(el,selector);assert.equal(el.disabled,false);el.click();
};
assert.equal(doc.querySelector('.app-header nav [aria-current]').textContent,'Live');
assert.equal(doc.querySelectorAll('.stream-tile').length,5);
checkText('Lounge simulator');checkText('No active session');
wc('connected-filter','all');assert.equal(doc.querySelectorAll('.stream-tile').length,6);checkText('No signal');
const scope=doc.querySelector('[data-watch-field="scope"]');scope.value='standby';scope.dispatchEvent(new w.Event('change',{bubbles:true}));
assert.equal(doc.querySelectorAll('.stream-tile').length,1,'an idle simulator remains watchable');
wc('open-source','lounge');assert.ok(doc.querySelector('dialog').textContent.includes('Camera is on'));click('close');
route('watch/club');assert.equal(doc.querySelectorAll('.map-driver').length,6);
assert.equal(read('watchState().focus'),'nora');
wc('focus-driver','jonas');assert.equal(read('watchState().source'),'rig-02');
assert.equal(doc.querySelector('.map-driver.selected').dataset.value,'jonas');
assert.equal(doc.querySelector('.camera-identity h3').textContent,'Jonas Berg');
wc('select-source','spectator');assert.equal(read('watchState().focus'),'');
assert.equal(doc.querySelectorAll('.map-driver.selected').length,0);
assert.equal(doc.querySelector('#watch-speed').textContent,'—','a spectator camera must not borrow a driver’s telemetry');
wc('focus-driver','erik');checkText('Rig 04 has no signal');
assert.equal(doc.querySelectorAll('.map-driver').length,6,'camera failure does not remove position telemetry');
wc('focus-driver','leo');checkText('No stream linked yet');
assert.equal(doc.querySelector('.map-driver.selected').dataset.value,'leo');
assert.equal(doc.querySelector('[data-watch-action="snapshot"]').disabled,true);
wc('new-source','leo');assert.equal(doc.querySelector('[name="driver"]').value,'leo');
wc('sample-source');wc('check-source');
let form=doc.querySelector('[data-watch-form="source"]');
assert.equal(form.querySelector('[type="submit"]').disabled,false);
form.elements.url.value='javascript:alert(1)';form.elements.url.dispatchEvent(new w.Event('input',{bubbles:true}));
assert.equal(form.querySelector('[type="submit"]').disabled,true,'edits invalidate the simulated connection check');
wc('check-source');assert.ok(form.querySelector('.error').textContent.includes('http://'));
wc('sample-source');form.elements.capture.value='https://simulator.example/rig/whep';wc('check-source');
assert.ok(form.querySelector('.error').textContent.includes('WHEP player'));
wc('sample-source');wc('check-source');form.dispatchEvent(new w.Event('submit',{bubbles:true,cancelable:true}));
assert.equal(read("sourceForDriver('leo').name"),'New simulator');
assert.equal(doc.querySelector('.camera-identity h3').textContent,'Leo Strand','saving a source immediately resolves the missing camera');
wc('focus-driver','leo');checkText('New simulator');
assert.equal(doc.querySelector('[data-watch-action="snapshot"]').disabled,false);
wc('snapshot',read('watchState().source'));assert.equal(read('watchState().captures[0].driver'),'leo');
route('driver/leo');wc('driver-captures','leo');assert.ok(doc.querySelector('dialog').textContent.includes('Leo Strand'));click('close');

// A shared rig keeps its camera, closes the old clip, and starts new attribution.
route('watch/club');wc('focus-driver','nora');wc('record','rig-01');
assert.equal(read("watchState().recording['rig-01'].driver"),'nora');
route('live/club');click('handover','club');click('assign','alex');
assert.equal(read("sourceDriver(getSource('rig-01')).id"),'alex');
assert.equal(read('watchState().captures[0].driver'),'nora','handover saves the previous driver’s clip');
assert.equal(read("watchState().recording['rig-01']"),undefined);
route('watch/club');assert.equal(read('watchState().focus'),'alex');
wc('source-settings','rig-01');assert.equal(doc.querySelector('select[name="driver"]').disabled,true);
assert.equal(read('sourceValues(document.querySelector("[data-watch-form]")).driver'),'alex');click('close');
wc('record','rig-01');route('live/club');click('end','club');click('confirm-end','club');route('watch/club');
checkText('This session has finished');assert.equal(read('watchState().captures[0].driver'),'alex');
assert.equal(read('Object.keys(watchState().recording).length'),0);
route('streams');read("watchState().scope='all';render()");assert.equal(doc.querySelectorAll('.stream-tile').length,7,'finishing a session leaves its camera sources available');
assert.equal(read('JSON.parse(sessionStorage.getItem(storageKey)).watch.captures.length'),3);

// Sample track positions use each installed map’s coordinate system.
click('reset');route('watch/club');
for(const trackName of ['rudskogen','ebisu','playground']){
  read(`findSession('club').track='${trackName}';render()`);
  assert.ok(doc.querySelector('.watch-map-canvas img').getAttribute('src').endsWith(trackName+'-map.png'));
  assert.equal(read('WATCH_MAPS[findSession("club").track].points.every(p=>p.every(x=>Number.isFinite(x)&&x>=-.02&&x<=1.02))'),true);
}
read("findSession('club').type='Race';render()");checkText('Same lap');
read("findSession('club').track='missing-layout';render()");checkText('Map unavailable for this layout');
read("findSession('club').track='rudskogen';render()");
const previousPoint=doc.querySelector('.map-driver').style.left;
assert.equal(intervals.size,1,'Watch runs one position updater');
for(let i=0;i<5;i++)for(const callback of intervals.values())callback();
assert.equal(read('watchState().tick'),5);
assert.notEqual(doc.querySelector('.map-driver').style.left,previousPoint,'the focused driver moves on the map');
wc('telemetry');checkText('Position updates delayed');assert.equal(intervals.size,0);
assert.ok(doc.querySelector('.frame-signal').textContent.includes('Connected'),'telemetry loss does not imply camera loss');
wc('telemetry');assert.equal(intervals.size,1);wc('motion');assert.equal(intervals.size,0,'pause stops animation');
route('streams');assert.equal(intervals.size,0,'leaving Watch stops animation');
dom.window.close();
console.log('PASS: session creation, capacity/conflict checks, scheduling, isolation, handover, recaps, search, first use, race phases, camera wall and filters, stream setup/validation, linked camera-map focus, spectator/offline/missing feeds, driver capture ownership, handover/end recording boundaries, persistence, all track maps, telemetry delay and animation cleanup.');
