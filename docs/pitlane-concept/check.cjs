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
w.eval(['map-data.js','garage.js','advanced.js','watch.js','app.js'].map(file=>fs.readFileSync(path.join(dir,file),'utf8')).join('\n')+'\nwindow.__read = expression => eval(expression);');
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
// Garage: rigs own their equipment, displays, and sources; drivers stay independent.
click('reset');route('garage');checkText('Your servers');checkText('The club');
assert.equal(doc.querySelectorAll('.garage-destination').length,2);
const gc=(action,value)=>{const el=doc.querySelector(`[data-garage-action="${action}"]${value!==undefined?`[data-value="${value}"]`:''}`);assert.ok(el,action+' '+value);assert.notEqual(el.disabled,true);el.click();};
const ac=(action,value)=>{const el=doc.querySelector(`[data-advanced-action="${action}"]${value!==undefined?`[data-value="${value}"]`:''}`);assert.ok(el,action+' '+value);assert.notEqual(el.disabled,true);el.click();};
const gs=type=>doc.querySelector(`[data-garage-form="${type}"]`).dispatchEvent(new w.Event('submit',{bubbles:true,cancelable:true}));
const as=type=>doc.querySelector(`[data-advanced-form="${type}"]`).dispatchEvent(new w.Event('submit',{bubbles:true,cancelable:true}));
gc('open-server','club-01');route('advanced/servers');assert.equal(read('advancedScope().name'),'Club 01');
route('rigs');assert.equal(doc.querySelectorAll('.rig-card').length,5);gc('edit-rig','');
form=doc.querySelector('[data-garage-form="rig"]');form.elements.name.value='Rig 01';gs('rig');
assert.ok(form.querySelector('.error').textContent.includes('already exists'));
form.elements.name.value='Studio rig';form.elements.location.value='Studio';form.querySelector('[name="display"][value="vr"]').checked=true;form.elements.driver.value='leo';gs('rig');
const studio=read("garageState().rigs.find(r=>r.name==='Studio rig').id");route('rig/'+studio);checkText('VR');
gc('edit-gear',studio);form=doc.querySelector('[data-garage-form="gear"]');form.elements.type.value='VR headset';form.elements.name.value='Test headset';form.elements.notes.value='Spare cable on shelf';gs('gear');checkText('Test headset');
const gearId=read(`getRig('${studio}').gear[0].id`);gc('edit-gear',studio+'/'+gearId);form=doc.querySelector('[data-garage-form="gear"]');form.elements.name.value='Studio headset';gs('gear');checkText('Studio headset');
gc('rig-source',studio);form=doc.querySelector('[data-watch-form="source"]');assert.equal(read('sourceValues(document.querySelector("[data-watch-form]")).driver'),'leo');wc('sample-source');wc('check-source');form.dispatchEvent(new w.Event('submit',{bubbles:true,cancelable:true}));
assert.equal(read(`rigSources(getRig('${studio}')).length`),1);
const studioSource=read(`rigSources(getRig('${studio}'))[0].id`);
assert.equal(read(`sourceDriver(getSource('${studioSource}')).id`),'leo');
gc('edit-rig',studio);form=doc.querySelector('[data-garage-form="rig"]');form.elements.driver.value='mia';form.querySelector('[name="display"][value="triple"]').checked=true;gs('rig');route('rig/'+studio);checkText('Triple screens');assert.equal(read(`sourceDriver(getSource('${studioSource}')).id`),'mia','rig association supplies camera identity');
gc('edit-gear',studio+'/'+gearId);gc('remove-gear',studio+'/'+gearId);assert.equal(read(`getRig('${studio}').gear.length`),0);
route('rigs');gc('rig-tab','cameras');assert.equal(doc.querySelectorAll('.stream-tile').length,1,'linked rig feeds do not appear as standalone cameras');

// Library: browse all types, edit labels, archive/restore, and review an import.
route('library');assert.equal(doc.querySelectorAll('.library-item').length,6);gc('content-tab','tracks');assert.equal(doc.querySelectorAll('.library-item').length,3);
route('content/rudskogen');gc('content-edit','rudskogen');form=doc.querySelector('[data-garage-form="content"]');form.elements.tags.value='Club favourite';gs('content');checkText('Club favourite');gc('archive-content','rudskogen');assert.equal(read("garageState().content.find(c=>c.id==='rudskogen').archived"),true);assert.equal(read("findSession('club').track"),'rudskogen','archiving does not rewrite session references');gc('restore-content','rudskogen');
route('library');gc('import-content');form=doc.querySelector('[data-garage-form="import"]');form.elements.name.value='Imported pack';form.elements.url.value='file:///private/demo.zip';gs('import');assert.ok(form.querySelector('.error').textContent.includes('http://'));
gc('sample-import');gs('import');assert.ok(doc.querySelector('#dialog-title').textContent.includes('arrive'));gc('confirm-import');route('library');checkText('Weekend car pack');assert.equal(doc.querySelectorAll('.library-item').length,7);
gc('import-content');gc('sample-import');gs('import');gc('confirm-import');assert.ok(doc.querySelector('#dialog-content .error').textContent.includes('replace'));doc.querySelector('#import-overwrite').checked=true;gc('confirm-import');route('library');assert.equal(doc.querySelectorAll('.library-item').length,7,'replacing a sample import does not duplicate it');

// Every advanced tool opens a concrete editor, diagnosis, review, or relevant workspace.
route('advanced');assert.ok(read('ADVANCED_TOOLS.length')>=50);
const ids=read('ADVANCED_TOOLS.map(t=>t.id)');
for(const id of ids){route('advanced');read(`openAdvanced('${id}')`);if(doc.querySelector('dialog').open)assert.ok(doc.querySelector('#dialog-title').textContent.length>0);else assert.ok(w.location.hash.length>1);if(doc.querySelector('dialog').open)click('close');}
route('advanced');const advancedSearch=doc.querySelector('[data-advanced-search]');advancedSearch.value='multiplier';advancedSearch.dispatchEvent(new w.Event('input',{bubbles:true}));checkText('Drift scoring modes');
ac('section','servers');route('advanced/servers');assert.equal(read('garageState().advanced.query'),'');
read("garageState().advanced.scope='club-02';render()");ac('tool','new-instance');form=doc.querySelector('[data-advanced-form="settings"]');form.elements.udp_port.value='9600';as('settings');assert.ok(form.querySelector('.error').textContent.includes('already in use'));form.elements.udp_port.value='9608';as('settings');assert.equal(doc.querySelector('#dialog-title').textContent,'Know what will change.');assert.equal(read('garageState().servers.length'),2,'review is not an applied mutation');ac('apply');assert.equal(read('garageState().servers.length'),3);
read("state.sessions.push({...findSession('club'),id:'undo-guard',server:garageState().servers.at(-1).name});openAdvanced('change-history')");ac('undo');assert.equal(read('garageState().servers.length'),3,'undo cannot remove a now-running server');read("state.sessions=state.sessions.filter(s=>s.id!=='undo-guard')");ac('undo');assert.equal(read('garageState().servers.length'),2);click('close');
read("garageState().advanced.scope='club-01';openAdvanced('instance')");as('settings');assert.ok(doc.querySelector('dialog .error').textContent.includes('Stop this server'));click('close');
read("garageState().advanced.scope='club-02';openAdvanced('grid')");form=doc.querySelector('[data-advanced-form="settings"]');form.elements.grid_entries.value='not json';as('settings');assert.ok(form.querySelector('.error').textContent.includes('valid JSON'));form.elements.grid_entries.value='[{"car":"s14","count":0,"ballast":0}]';as('settings');assert.ok(form.querySelector('.error').textContent.includes('positive whole count'));click('close');
read("openAdvanced('custom-session')");form=doc.querySelector('[data-advanced-form="settings"]');form.elements.session_name.value='My custom setup';form.elements.grid_size.value='24';as('settings');assert.ok(form.querySelector('.error').textContent.includes('pit spaces'));form.elements.grid_size.value='12';as('settings');ac('apply');route('build');checkText('My custom setup');checkText('Custom setup details');assert.equal(read('draft.customConfig.layout'),'Default');
route('advanced/custom');ac('tool','raw-config');form=doc.querySelector('[data-advanced-form="raw"]');assert.ok(form.elements.server.value.includes('My custom setup'));form.elements.server.value='[SERVER]\ninvalid line';as('raw');assert.ok(form.querySelector('.error').textContent.includes('KEY=value'));form.elements.server.value='[SERVER]\nNAME=Edited draft';as('raw');ac('apply');ac('tool','raw-config');assert.ok(doc.querySelector('[name="server"]').value.includes('Edited draft'),'raw draft is retained');click('close');
route('advanced/recovery');ac('tool','cleanup');ac('repair','cleanup');assert.equal(read('garageState().advanced.cleaned'),true);read("openAdvanced('change-history')");ac('undo');assert.equal(read('garageState().advanced.cleaned'),false);click('close');ac('tool','restore');ac('stage-restore');assert.equal(read('garageState().advanced.stagedRestore'),true);read("openAdvanced('change-history')");ac('undo');assert.equal(read('garageState().advanced.stagedRestore'),false);click('close');
route('advanced/automation');ac('tool','queue-editor');ac('queue-add');assert.equal(doc.querySelectorAll('.queue-demo-row').length,2);const addedQueue=read("state.sessions.find(s=>s.id.startsWith('queue-')).id");ac('queue-up',addedQueue);assert.ok(doc.querySelector('.queue-demo-row b').textContent.includes('Next up'));click('close');ac('tool','repeat');form=doc.querySelector('[data-advanced-form="settings"]');form.elements.run_mode.value='Repeat event';as('settings');ac('apply');ac('tool','queue-editor');assert.equal(doc.querySelectorAll('.queue-demo-row').length,0);assert.ok(doc.querySelector('dialog').textContent.includes('manual queue is waiting'));click('close');
assert.equal(read('JSON.parse(sessionStorage.getItem(storageKey)).garage.rigs.some(r=>r.name==="Studio rig")'),true);

dom.window.close();
console.log('PASS: session creation, capacity/conflict checks, scheduling, isolation, handover, recaps, search, first use, race phases, camera wall and filters, stream setup/validation, linked camera-map focus, spectator/offline/missing feeds, driver capture ownership, handover/end recording boundaries, persistence, all track maps, telemetry delay and animation cleanup; Garage rig/display/gear CRUD, source ownership, content search/import/archive, all advanced tools, scoped review, port conflicts, custom/INI drafts, cleanup/restore undo, queue ordering/repeat and persistence.');
