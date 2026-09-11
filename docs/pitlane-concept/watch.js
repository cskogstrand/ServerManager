// Watch interaction study. Camera frames, source checks, and telemetry are simulated.
// No source URLs are fetched; the production WHEP/embed/capture implementations stay untouched.
function newWatchState() {
  return {
    focus:'nora', source:'rig-01', connectedOnly:true, scope:'all', motion:true, stale:false, tick:0,
    sources:[
      {id:'rig-01',name:'Rig 01',driver:'nora',status:'live',image:'silvia.jpg',url:'https://simulator.example/rig-01/whep',capture:'rtsp://simulator.example/rig-01',auto:true},
      {id:'rig-02',name:'Rig 02',driver:'jonas',status:'live',image:'rudskogen.png',url:'https://simulator.example/rig-02/whep',capture:'rtsp://simulator.example/rig-02',auto:true},
      {id:'rig-03',name:'Rig 03',driver:'mia',status:'live',image:'playground.png',url:'https://simulator.example/rig-03/whep',capture:'',auto:false},
      {id:'lounge',name:'Lounge simulator',driver:'',status:'live',image:'ebisu.png',url:'https://simulator.example/lounge/whep',capture:'',auto:false},
      {id:'rig-04',name:'Rig 04',driver:'erik',status:'offline',image:'rudskogen.png',url:'https://simulator.example/rig-04/whep',capture:'rtsp://simulator.example/rig-04',auto:true},
      {id:'spectator',name:'Trackside camera',driver:'',session:'club',status:'live',image:'rudskogen.png',url:'https://simulator.example/trackside/',capture:'',auto:false},
    ],
    captures:[], recording:{}, sequence:0,
  };
}
function watchState(){return state.watch ||= newWatchState();}
function sourceDriver(source){const rig=rigForSource(source);return rig?rigDriver(rig):state.drivers.find(d=>d.id===source?.driver);}
function sourceSession(source){
  if(!source)return null;
  if(source.session)return state.sessions.find(s=>s.id===source.session&&s.status==='live');
  const d=sourceDriver(source);
  return d?.live?state.sessions.find(s=>s.status==='live'&&s.id===(d.session||'club')):null;
}
function sessionSources(s){return watchState().sources.filter(source=>sourceSession(source)?.id===s.id);}
function getSource(id){return watchState().sources.find(source=>source.id===id);}
function sourceName(source){return sourceDriver(source)?.name || (source.session?'Spectator view':'Ready for a driver');}
function sourceHealth(source){return source.status==='live'?'Connected':source.status==='offline'?'No signal':'Checking signal';}
function sourceForDriver(id){if(!id)return undefined;return watchState().sources.find(source=>sourceDriver(source)?.id===id);}
function mapPosition(s,index,tick=watchState().tick){
  const data=WATCH_MAPS[s.track];if(!data)return null;
  const phase=(.09+index*.147+tick*.0009)%1;
  const n=phase*data.points.length,i=Math.floor(n),a=data.points[i],b=data.points[(i+1)%data.points.length],f=n-i;
  return {x:(a[0]+(b[0]-a[0])*f)*100,y:(a[1]+(b[1]-a[1])*f)*100,speed:Math.round(58+37*Math.sin(phase*Math.PI)**2),gear:3,rpm:Math.round(5100+phase*1600)};
}
function watchMap(s){
  const ws=watchState(),data=WATCH_MAPS[s.track],ds=sessionDrivers(s);
  return `<section class="watch-map-panel" aria-labelledby="map-title"><div class="watch-panel-head"><h2 id="map-title">On the circuit</h2><span class="signal ${ws.stale?'warning':''}">${ws.stale?'Last position held':'Positions · demo'}</span></div>
    ${data?`<div class="watch-map-stage"><div class="watch-map-canvas" style="--map-ratio:${data.width/data.height}"><img src="${asset(s.track+'-map.png')}" alt="${e(track(s).name)} track map">${ds.map((d,i)=>{const p=mapPosition(s,i);return `<button class="map-driver ${ws.focus===d.id?'selected':''}" data-watch-action="focus-driver" data-value="${d.id}" data-index="${i}" style="left:${p.x}%;top:${p.y}%" aria-label="Follow ${e(d.name)} on the map" aria-pressed="${ws.focus===d.id}"><span>${i+1}</span></button>`;}).join('')}</div></div>`:`<div class="watch-map-empty">${icon('road')}<h3>Map unavailable for this layout.</h3><p>Streams and timing are still available.</p></div>`}
    <div class="watch-map-footer"><b>${e(track(s).name)}</b><span>${ds.length} drivers · tap a number to follow</span></div>
    ${ws.stale?'<div class="watch-map-warning" role="status">Position updates delayed. Showing the last known positions; camera status is independent.</div>':''}
    ${!ds.length?'<p class="watch-empty-note">Waiting for the first driver to join this session.</p>':''}
  </section>`;
}
function cameraFrame(source,large=false){
  const d=source&&sourceDriver(source),ws=watchState(),active=source&&source.status==='live';
  if(!source)return `<div class="camera-frame no-signal ${large?'large':''}">${icon('screen')}<h3>No stream linked yet.</h3><p>This driver is on the map. Add their simulator feed to watch them here.</p><button class="button light" data-watch-action="new-source" data-value="${e(ws.focus)}">Set up this simulator ${icon('plus')}</button></div>`;
  return `<div class="camera-frame ${large?'large':''} ${active?'':'no-signal'}" data-source-frame="${source.id}">
    ${active?`<img src="${asset(source.image)}" alt="${e(source.name)} sample camera frame"><div class="camera-shade"></div><span class="frame-label">SAMPLE FRAME</span><span class="frame-signal">${ws.recording[source.id]?'● Recording demo':'● Connected · demo'}</span><div class="camera-identity"><span class="eyebrow">${e(source.name)}${source.session?' / TRACKSIDE':''}</span><h3>${e(sourceName(source))}</h3><p>${d?e(d.car):'Camera is on · no game connection needed'}</p></div>`:`${icon('screen')}<h3>${e(source.name)} has no signal.</h3><p>${d?.live?e(d.name)+' is still driving. Timing and map positions continue.':'The source stays configured, ready to reconnect.'}</p><button class="button light compact" data-watch-action="retry-source" data-value="${source.id}">Retry sample connection</button>`}
  </div>`;
}
function sourceTile(source,compact=false){
  const ws=watchState(),session=sourceSession(source),selected=ws.source===source.id;
  return `<article class="stream-tile ${compact?'compact':''} ${selected?'selected':''}"><button class="stream-open" data-watch-action="${compact?'select-source':'open-source'}" data-value="${source.id}" aria-label="Watch ${e(source.name)}${sourceDriver(source)?' · '+e(sourceName(source)):''}" ${compact?`aria-pressed="${selected}"`:''}>
    <div class="tile-image ${source.status==='live'?'':'offline'}">${source.status==='live'?`<img src="${asset(source.image)}" alt=""><span class="tile-stamp">Sample frame</span>`:`<span>${icon('screen')} ${sourceHealth(source)}</span>`}<span class="tile-focus">${icon('play')}</span></div>
    <div class="tile-info"><span class="row between"><b>${e(source.name)}</b><span class="signal ${source.status==='live'?'':'warning'}">${sourceHealth(source)}</span></span><span class="tile-driver">${e(sourceName(source))}</span><span class="tile-session">${session?e(session.name):'No active session · camera available'}</span></div></button>
    ${compact?'':`<div class="tile-actions"><button data-watch-action="source-settings" data-value="${source.id}">Stream settings ${icon('sliders')}</button><button data-watch-action="expand-source" data-value="${source.id}" aria-label="Enlarge ${e(source.name)}">${icon('screen')}</button></div>`}
  </article>`;
}
function watchBoard(s){const ws=watchState(),ds=sessionDrivers(s);return `<section class="watch-board"><div class="watch-panel-head"><h2>${s.type==='Drift'?'The drift board':'Live timing'}</h2><span class="small muted">Select a driver to follow</span></div><table class="timing"><caption class="sr-only">${e(s.name)} ${s.type==='Drift'?'drift scores':'lap times'}; selecting a driver links their stream and map position</caption><thead><tr><th>#</th><th>Driver / camera</th><th>${s.type==='Drift'?'Live run':'Best lap'}</th><th>${s.type==='Drift'?'Best run':'Gap'}</th></tr></thead><tbody>${ds.map((d,i)=>{const source=sourceForDriver(d.id);return `<tr class="${ws.focus===d.id?'following':''}"><td>${i+1}</td><td><button class="person-button" data-watch-action="focus-driver" data-value="${d.id}" aria-pressed="${ws.focus===d.id}"><span class="name">${e(d.name)}</span><span class="car">${source?e(source.name)+' · '+sourceHealth(source):'No stream linked'}${d.id===state.seat?' · shared rig':''}</span></button></td><td class="num">${s.type==='Drift'?fmt(Math.round(d.score*.64)):e(d.lap)}</td><td class="num">${s.type==='Drift'?fmt(d.score):s.type==='Race'?'Same lap':i?'+'+(i*1.37).toFixed(3):'Leader'}</td></tr>`;}).join('')||'<tr><td colspan="4">Waiting for drivers. Connected cameras remain in All live streams.</td></tr>'}</tbody></table></section>`;}
function watchSession(id){
  const s=findSession(id);if(!s)return missing();
  if(s.status==='planned')return heading('This session hasn’t started.','Connected cameras are still available in Live.',linkBtn('All live streams','streams')+linkBtn('Session plan','sessions','primary'));
  if(s.status!=='live')return `${heading('This session has finished.','Your results are saved. Cameras may still be connected.',linkBtn('All live streams','streams')+linkBtn('View recap','recap/'+id,'primary'))}`;
  const ws=watchState(),ds=sessionDrivers(s),sources=sessionSources(s);let source=getSource(ws.source);
  if(ws.session!==id||source&&sourceSession(source)?.id!==id){
    ws.session=id;
    if(!source||sourceSession(source)?.id!==id)source=sourceForDriver(ds[0]?.id)||sources[0];
    ws.source=source?.id||'';ws.focus=source?sourceDriver(source)?.id||'':ds[0]?.id||'';
  }else if(source){ws.focus=sourceDriver(source)?.id||'';}
  else if(!ds.some(d=>d.id===ws.focus)){ws.focus=ds[0]?.id||'';source=sourceForDriver(ws.focus);ws.source=source?.id||'';}
  const d=ds.find(d=>d.id===ws.focus),p=d?mapPosition(s,Math.max(0,ds.indexOf(d))):null;
  return `<div id="watch-workspace"><header class="watch-session-header"><div><a class="back" href="#live/${id}">${icon('back')} Session control</a><div class="row"><span class="badge live"><span class="status-dot"></span>On track</span><span class="small muted">${e(s.server)} · ${e(track(s).name)}</span></div><h1>${e(s.name)}</h1></div><div class="watch-header-actions">${linkBtn(icon('screen')+' All live streams','streams','light')}<button class="button light" data-watch-action="setup">${icon('sliders')} Set up streams</button><button class="button light" data-watch-action="fullscreen">${icon('screen')} Fullscreen</button></div></header>
  <div class="watch-demo-strip"><span>Interactive concept · sample camera frames & simulated positions</span><div><button data-watch-action="motion">${ws.motion?'Pause':'Resume'} positions</button><button data-watch-action="telemetry">${ws.stale?'Restore telemetry':'Try telemetry delay'}</button></div></div>
  <div class="watch-stage"><section class="featured-camera"><div class="watch-panel-head"><h2>${source?e(source.name):d?e(d.name):'Choose a simulator'}</h2><div class="row">${source?`<button class="text-button" data-watch-action="source-settings" data-value="${source.id}" aria-label="Set up ${e(source.name)}">${icon('sliders')} Stream setup</button>`:''}</div></div>${cameraFrame(source,true)}
    <div class="driver-instruments"><span><b id="watch-speed" class="num">${p?p.speed:'—'}</b> km/h</span><span>Gear <b>${p?p.gear:'—'}</b></span><span><b id="watch-rpm" class="num">${p?fmt(p.rpm):'—'}</b> rpm</span><span class="instrument-note">${d?e(d.name.split(' ')[0])+' · '+(ws.stale?'last telemetry held':'simulated telemetry'):'Spectator camera'}</span></div>
    <div class="camera-controls"><div class="row"><button class="button light compact" data-watch-action="snapshot" data-value="${source?.id||''}" ${source?.status==='live'&&source.capture?'':'disabled'}>${icon('camera')} Snapshot</button><button class="button ${source&&ws.recording[source.id]?'primary':'light'} compact" data-watch-action="record" data-value="${source?.id||''}" ${source?.status==='live'&&source.capture?'':'disabled'}>${source&&ws.recording[source.id]?'Stop & save':'Record clip'}</button></div><button class="text-button" data-watch-action="captures">Saved moments <span class="badge">${ws.captures.length}</span></button></div>
    <p class="capture-hint">${source?.capture?source.auto?'Automatic drift highlights are on. You can also save a moment yourself.':'Recording source connected. Capture a moment whenever you want.':'Watching and recording are separate. Add a recording source in Stream setup to save clips.'}</p>
  </section>${watchMap(s)}</div>
  <div class="watch-lower"><section><div class="watch-panel-head"><h2>From the simulators</h2><span class="small muted">${sources.filter(x=>x.status==='live').length} connected · ${sources.length} linked to this session</span></div><div class="camera-strip">${sources.map(x=>sourceTile(x,true)).join('')||'<div class="watch-map-empty"><p>No cameras linked to these drivers yet.</p><button class="button light" data-watch-action="setup">Set up streams</button></div>'}</div><p class="watch-small-note">The camera belongs to the simulator. Its label follows the person at the wheel.</p></section>${watchBoard(s)}</div></div>`;
}
function liveStreams(){const ws=watchState();const filtered=ws.sources.filter(source=>(!ws.connectedOnly||source.status==='live')&&(ws.scope==='all'||(ws.scope==='standby'?!sourceSession(source):sourceSession(source)?.id===ws.scope)));const live=state.sessions.filter(s=>s.status==='live');return `${heading('Every simulator. One view.','See every connected feed, whether its driver is on track or getting ready.',`<button class="button primary" data-watch-action="setup">${icon('plus')} Set up streams</button>`,'LIVE / ACROSS YOUR CLUB')}
  <div class="stream-summary"><span class="badge live"><span class="status-dot"></span>${ws.sources.filter(x=>x.status==='live').length} cameras connected</span><span>${ws.sources.filter(x=>x.status!=='live').length} awaiting signal</span><span>${live.length} ${live.length===1?'session':'sessions'} on track</span><span class="stream-demo-note">Sample frames · simulated connections</span></div>
  <div class="stream-filter"><div class="segmented" aria-label="Stream visibility"><button data-watch-action="connected-filter" data-value="connected" aria-pressed="${ws.connectedOnly}">Connected now</button><button data-watch-action="connected-filter" data-value="all" aria-pressed="${!ws.connectedOnly}">All sources</button></div><label class="stream-scope">Show<select data-watch-field="scope"><option value="all">All sessions & simulators</option><option value="standby" ${ws.scope==='standby'?'selected':''}>Not in a session</option>${live.map(s=>`<option value="${s.id}" ${ws.scope===s.id?'selected':''}>${e(s.name)}</option>`).join('')}</select></label></div>
  <div class="all-stream-grid">${filtered.map(x=>sourceTile(x)).join('')||'<div class="empty"><h2>No feeds in this view.</h2><p>Show all sources to see what needs connecting.</p><button class="button primary" data-watch-action="connected-filter" data-value="all">Show all sources</button></div>'}</div>
  ${live.length?`<div class="section-heading"><h2>Watch a whole session</h2><p class="small">Video, track positions, and timing together.</p></div><div class="live-session-links">${live.map(s=>`<a class="live-session-link" href="#watch/${s.id}"><img src="${asset(track(s).image)}" alt=""><span><b>${e(s.name)}</b><small>${e(s.server)} · ${e(track(s).name)} · ${s.people} drivers</small></span>${icon('arrow')}</a>`).join('')}</div>`:`<div class="empty"><h2>No sessions on track.</h2><p>Connected cameras remain available above. Browse the club’s previous results while you wait.</p>${linkBtn('Sessions & results','sessions')}</div>`}`;}
function setupStreams(){const ws=watchState();modal('Your simulators, connected.',`<p>Link a feed once. Watch it here and from any session its driver joins.</p><div class="stream-manager">${ws.sources.map(source=>`<div class="managed-source"><span class="setting-icon">${icon('screen')}</span><span><b>${e(source.name)}</b><small>${e(sourceName(source))} · ${sourceHealth(source)}</small></span><button class="button compact" data-watch-action="source-settings" data-value="${source.id}" aria-label="Edit ${e(source.name)}">Edit</button></div>`).join('')}</div><div class="dialog-actions">${linkBtn('Rigs, gear & displays','rigs')}<button class="button primary" data-watch-action="new-source">${icon('plus')} Add camera feed</button></div><p class="notice">Demo sources only. Playback, recording, and driver presence have separate statuses.</p>`);}
function sourceEditor(id,driver='',rigId=''){
  const source=getSource(id),linkedRig=getRig(rigId)||rigForSource(source),next=source||{name:linkedRig?linkedRig.name+' camera':'',driver,status:'unknown',url:'',capture:'',auto:false};
  modal(source?'Set up '+e(source.name):'Connect a simulator.',`<p>A name and a player link are enough to watch. Recording is optional.</p><form data-watch-form="source" data-source-id="${e(id||'')}" data-checked="false"><label class="field">Simulator or camera name<input name="name" value="${e(next.name)}" placeholder="Rig 05" required maxlength="50"></label><label class="field">Player URL<input name="url" type="url" value="${e(next.url)}" placeholder="https://your-stream-server/rig-05/whep" required><small>WebRTC / WHEP or an embeddable player page.</small></label>${linkedRig?`<input name="rig" type="hidden" value="${linkedRig.id}"><p class="garage-inline-note">Attached to <a class="text-button" href="#rig/${linkedRig.id}">${e(linkedRig.name)} ${icon('arrow')}</a> · gear and display setup live on the rig page.</p>`:`<label class="field">Attach to a rig<select name="rig"><option value="">Standalone camera / no rig yet</option>${garageState().rigs.map(r=>`<option value="${r.id}">${e(r.name)}</option>`).join('')}</select><small>Optional. A linked rig supplies the driver identity.</small></label>`}<label class="field source-driver-field">Who uses this simulator?<select name="driver" ${linkedRig?'disabled':''}><option value="">No driver assigned / spectator camera</option>${state.drivers.map(d=>`<option value="${d.id}" ${d.id===(linkedRig?rigDriver(linkedRig)?.id:next.driver)?'selected':''}>${e(d.name)}</option>`).join('')}</select>${linkedRig?`<input type="hidden" name="driver" value="${e(rigDriver(linkedRig)?.id||'')}">`:''}<small>${linkedRig?linkedRig.id==='rig-01'?'Use Next driver in Session control to hand over this shared rig. Its camera stays linked.':'The driver comes from this rig’s profile.':'The feed follows this driver into the session they join.'}</small></label><label class="field spectator-session" ${next.driver?'hidden':''}>Session for an unassigned camera<select name="session"><option value="">None · available in the live wall</option>${state.sessions.filter(s=>s.status==='live').map(s=>`<option value="${s.id}" ${s.id===next.session?'selected':''}>${e(s.name)}</option>`).join('')}</select><small>Optional. Use this for a trackside or spectator view.</small></label>
    <details class="advanced" ${next.capture?'open':''}><summary>Record clips & save highlights</summary><label class="field">Recording source URL<input name="capture" value="${e(next.capture)}" placeholder="rtsp://your-stream-server/rig-05"><small>Use a separate recording link: RTSP, RTMP, SRT or HLS. The WHEP player link is only for watching.</small></label><label class="toggle-line"><span>Automatically save great drift runs<small>Only active with a working recording source.</small></span><input name="auto" type="checkbox" ${next.auto?'checked':''}></label></details>
    <div class="error" role="alert"></div><div class="source-check" role="status"></div><div class="source-test-actions"><button type="button" class="text-button" data-watch-action="sample-source">Use sample source</button><button type="button" class="button" data-watch-action="check-source">Check sample connection</button></div><p class="small muted">Prototype check: validates the fields and simulates a connection. Your URLs are never contacted.</p><div class="dialog-actions">${btn('Cancel','close')}<button class="button primary" type="submit" disabled>Save stream</button></div></form>`);
}
function validateSource(values){
  if(!values.name.trim())return 'Give this simulator a name.';
  let player;try{player=new URL(values.url);}catch{return 'Enter a complete player URL.';}
  if(!['http:','https:'].includes(player.protocol))return 'The player link must use http:// or https://.';
  if(values.capture){let capture;try{capture=new URL(values.capture);}catch{return 'Enter a complete recording source URL.';}
    if(!['http:','https:','rtsp:','rtmp:','srt:'].includes(capture.protocol))return 'Use an HLS, RTSP, RTMP or SRT recording source.';
    if(/\/whep\/?$/i.test(capture.pathname))return 'That is a WHEP player URL. Use a raw recording source, such as RTSP or HLS.';
  }
  if(values.auto&&!values.capture)return 'Add a recording source, or turn off automatic highlights.';
  if(values.driver&&!state.drivers.some(d=>d.id===values.driver))return 'Select an existing driver.';
  return '';
}
function sourceValues(form){const f=new FormData(form),rig=getRig(String(f.get('rig')||'')),driver=rig?rigDriver(rig)?.id||'':String(f.get('driver')||'');return {name:String(f.get('name')||'').trim(),url:String(f.get('url')||'').trim(),rig:rig?.id||'',driver,session:driver?'':String(f.get('session')||''),capture:String(f.get('capture')||'').trim(),auto:f.get('auto')==='on'};}
function checkSource(){const form=document.querySelector('[data-watch-form="source"]');if(!form)return;const values=sourceValues(form),error=validateSource(values);form.querySelector('.error').textContent=error;form.dataset.checked=error?'false':'true';form.querySelector('[type="submit"]').disabled=!!error;form.querySelector('.source-check').innerHTML=error?'':`<div class="readiness"><div class="row">${icon('check')}Sample player connected</div><p>${values.capture?'Sample recorder connected. Photos and clips are available.':'Watching is ready. Add a recording source later if you want clips.'}</p></div>`;}
function captureMoment(source,kind,recording){const ws=watchState(),d=sourceDriver(source),s=sourceSession(source);const entry={id:++ws.sequence,kind,source:source.id,name:source.name,driver:recording?.driver??d?.id??'',driverName:recording?.driverName??d?.name??'Spectator camera',session:recording?.session??s?.id??'',image:source.image};ws.captures.unshift(entry);const session=findSession(entry.session);if(session)session.highlights++;remember();return entry;}
function finishSourceRecording(id){const ws=watchState(),recording=ws.recording[id];if(recording){captureMoment(getSource(id),'clip',recording);delete ws.recording[id];}}
function finishSessionCaptures(id){Object.entries(watchState().recording).forEach(([source,recording])=>{if(recording.session===id)finishSourceRecording(source);});}
function showCaptures(driver){const captures=watchState().captures.filter(c=>!driver||c.driver===driver);modal('Saved from the simulators.',`<p>Manual captures are filed with the person driving when recording began.</p>${captures.length?captures.map(c=>`<div class="managed-source"><img class="capture-thumb" src="${asset(c.image)}" alt="${e(c.name)} sample frame"><span><b>${e(c.driverName)}</b><small>${c.kind==='clip'?'Clip':'Snapshot'} · ${e(c.name)} · demo capture</small></span><button class="button compact" data-watch-action="preview-capture" data-value="${c.id}">View</button></div>`).join(''):'<div class="empty"><h3>The next good moment is yours.</h3><p>Use Snapshot or Record clip while watching a connected simulator.</p></div>'}`);}
function expandSource(source){modal(e(source.name),`${cameraFrame(source,true)}<p class="notice">Sample frame, not a live video connection. A connected WHEP player or embed occupies this space in the real app.</p><div class="dialog-actions"><button class="button primary" data-watch-action="source-settings" data-value="${source.id}">Stream setup</button></div>`);}
function selectSource(id){const source=getSource(id);if(!source)return;const ws=watchState();ws.source=id;const d=sourceDriver(source);ws.focus=d?.id||'';render();}
let watchTimer=null;
function syncWatchAnimation(){
  if(watchTimer)clearInterval(watchTimer);watchTimer=null;
  const [page,id]=location.hash.slice(1).split('/'),ws=watchState();
  const reduced=typeof matchMedia==='function'&&matchMedia('(prefers-reduced-motion: reduce)').matches;
  if(page!=='watch'||!ws.motion||ws.stale||document.hidden||reduced)return;
  const s=findSession(id);if(!s||s.status!=='live')return;
  // ponytail: animate sampled AI lines only; production uses the existing interpolated SSE positions.
  watchTimer=setInterval(()=>{
    ws.tick++;document.querySelectorAll('.map-driver').forEach(el=>{const p=mapPosition(s,Number(el.dataset.index));if(p){el.style.left=p.x+'%';el.style.top=p.y+'%';}});
    const i=sessionDrivers(s).findIndex(d=>d.id===ws.focus),p=i>=0?mapPosition(s,i):null;
    if(p){const speed=document.querySelector('#watch-speed'),rpm=document.querySelector('#watch-rpm');if(speed)speed.textContent=p.speed;if(rpm)rpm.textContent=fmt(p.rpm);}
  },250);
}
document.addEventListener('visibilitychange',syncWatchAnimation);
document.addEventListener('click',event=>{
  const button=event.target.closest('[data-watch-action]');if(!button)return;
  const action=button.dataset.watchAction,value=button.dataset.value,ws=watchState();
  if(action==='setup'){setupStreams();return;}
  if(action==='source-settings'){sourceEditor(value);return;}
  if(action==='new-source'){sourceEditor('',value||'');return;}
  if(action==='sample-source'){const form=button.closest('form');form.elements.name.value ||= 'New simulator';form.elements.url.value='https://simulator.example/new-rig/whep';form.elements.capture.value='rtsp://simulator.example/new-rig';form.dataset.checked='false';form.querySelector('[type="submit"]').disabled=true;form.querySelector('.source-check').textContent='Sample links added. Check the connection to continue.';return;}
  if(action==='check-source'){checkSource();return;}
  if(action==='select-source'){selectSource(value);return;}
  if(action==='focus-driver'){ws.focus=value;ws.source=sourceForDriver(value)?.id||'';render();return;}
  if(action==='open-source'){const source=getSource(value),s=sourceSession(source);if(s){ws.source=value;ws.focus=sourceDriver(source)?.id||'';go('watch/'+s.id);}else expandSource(source);return;}
  if(action==='expand-source'){expandSource(getSource(value));return;}
  if(action==='connected-filter'){ws.connectedOnly=value==='connected';render();return;}
  if(action==='motion'){ws.motion=!ws.motion;render();return;}
  if(action==='telemetry'){ws.stale=!ws.stale;render();return;}
  if(action==='retry-source'){const source=getSource(value);source.status='live';render();toast('Sample camera reconnected. Driver state is unchanged.');return;}
  if(action==='captures'){showCaptures();return;}
  if(action==='driver-captures'){showCaptures(value);return;}
  if(action==='preview-capture'){const c=ws.captures.find(c=>String(c.id)===value);modal(`${e(c.driverName)} · ${c.kind==='clip'?'clip':'snapshot'}`,`<img class="preview-frame" src="${asset(c.image)}" alt="Sample camera frame"><p>Demo capture from ${e(c.name)}. No real video or picture was recorded.</p>`);return;}
  if(action==='snapshot'||action==='record'){
    const source=getSource(value);if(!source||source.status!=='live'||!source.capture)return;
    if(action==='snapshot'){const c=captureMoment(source,'snapshot');render();toast(`Demo snapshot saved to ${c.driverName}.`);return;}
    if(ws.recording[value]){const c=captureMoment(source,'clip',ws.recording[value]);delete ws.recording[value];render();toast(`Demo clip saved to ${c.driverName}.`);}
    else{const d=sourceDriver(source);ws.recording[value]={driver:d?.id||'',driverName:d?.name||'Spectator camera',session:sourceSession(source)?.id||''};render();toast('Demo recording started. Stop to save the clip.');}
    return;
  }
  if(action==='fullscreen'){const el=document.querySelector('#watch-workspace');if(document.fullscreenElement){document.exitFullscreen?.();}else if(el?.requestFullscreen){el.requestFullscreen().catch(()=>toast('Fullscreen is unavailable in this browser.'));}else toast('Fullscreen is unavailable in this browser.');}
});
document.addEventListener('change',event=>{if(event.target.dataset.watchField==='scope'){watchState().scope=event.target.value;render();}});
function invalidateSourceCheck(event){const form=event.target.closest('[data-watch-form="source"]');if(!form)return;if(event.target.name==='rig'){const r=getRig(event.target.value);form.querySelector('.source-driver-field').hidden=!!r;form.querySelector('.spectator-session').hidden=!!r&&!!rigDriver(r);}if(event.target.name==='driver')form.querySelector('.spectator-session').hidden=!!event.target.value;form.dataset.checked='false';form.querySelector('[type="submit"]').disabled=true;form.querySelector('.source-check').textContent='Check the sample connection after making changes.';}
document.addEventListener('input',invalidateSourceCheck);document.addEventListener('change',invalidateSourceCheck);
document.addEventListener('submit',event=>{
  const form=event.target.closest('[data-watch-form="source"]');if(!form)return;event.preventDefault();
  const values=sourceValues(form),error=validateSource(values);if(error||form.dataset.checked!=='true'){form.querySelector('.error').textContent=error||'Check the sample connection before saving.';return;}
  const ws=watchState();let source=getSource(form.dataset.sourceId);
  if(source)Object.assign(source,values,{status:'live'});else{source={...values,id:'source-'+(++ws.sequence),status:'live',image:'silvia.jpg'};ws.sources.push(source);}
  if(location.hash==='#watch/'+sourceSession(source)?.id){ws.source=source.id;ws.focus=sourceDriver(source)?.id||'';}
  close();render();toast(`${source.name} saved to the concept. Sample feed available in Live.`);
});
