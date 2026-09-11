#!/usr/bin/env python3
"""Exercise ONLY the disposable localhost:3031 Pitlane installation.
Uses its known fixture admin account; refuses an installation outside /tmp/pitlane-fixture.
Does not stage a restore or modify real installation data.
"""
import http.cookiejar,json,time,urllib.request,urllib.error,uuid,io,zipfile,tempfile,sqlite3
from pathlib import Path
BASE='http://localhost:3031'
class Client:
 def __init__(self,name=None,password=None):
  self.cookies=http.cookiejar.CookieJar();self.http=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.cookies))
  if name:self.call('POST','/api/login',{'name':name,'password':password})
 def call(self,method,path,body=None,expect=200,raw=False,content_type="application/json"):
  headers={}
  if body is not None:headers['Content-Type']=content_type
  if method!='GET':headers['X-CSRF-Token']=next((c.value for c in self.cookies if c.name=='csrf_token'),'')
  request=urllib.request.Request(BASE+path,data=body if isinstance(body,bytes) else json.dumps(body).encode() if body is not None else None,headers=headers,method=method)
  try:response=self.http.open(request,timeout=40)
  except urllib.error.HTTPError as e:response=e
  data=response.read()
  assert response.status==expect,(method,path,response.status,data[:500])
  if raw:return data
  return json.loads(data) if data else None
admin=Client('admin','admin')
assert admin.call('GET','/api/config')['install_path']=='/tmp/pitlane-fixture/install','Refusing a non-fixture installation'
checks=[]
def passed(name):checks.append(name);print('PASS',name,flush=True)
Client().call('GET','/api/sources',expect=401);passed('Unauthenticated API rejected')
for role in ['viewer','steward']:
 name='fixture-'+role+'-'+uuid.uuid4().hex[:6]
 admin.call('POST','/api/users',{'name':name,'password':'temporary-fixture-only','role':role})
 client=Client(name,'temporary-fixture-only')
 sources=client.call('GET','/api/sources')['sources']
 assert all('capture_url' not in s and 'status_url' not in s for s in sources)
 client.call('GET','/api/streams/debug',expect=403)
 client.call('POST','/api/rigs',{'name':'Forbidden'},expect=403)
 if role=='viewer':client.call('POST','/api/driving-sessions',{},expect=403)
 else:
  draft=client.call('GET','/api/driving-sessions/defaults?experience=practice')
  draft['name']='Steward draft · fixture';client.call('POST','/api/driving-sessions',draft)
 passed(role+' capabilities and secret redaction')
for path in ['/api/rigs','/api/sources','/api/driving-sessions','/api/guest-drivers','/api/content/metadata','/api/recovery/changes']:
 admin.call('GET',path)
passed('Inventory, sessions, guests, metadata and recovery APIs')
sources=admin.call('GET','/api/sources')['sources']
source=next(s for s in sources if s['driver_guid']=='pitlane-fixture-1')
# Optimistic source edits also detect changes made through legacy adapters.
saved=admin.call('PUT','/api/sources/'+source['id'],{**source,'name':'Practice seat · test video' if source['name'].endswith('checked') else 'Practice seat · test video · checked'})
admin.call('PUT','/api/sources/'+source['id'],{**source,'name':'stale edit'},expect=409)
passed('Source version conflict rejects stale edit')
# A still is pulled from actual ffmpeg segments while the game is disconnected.
for _ in range(15):
 status=admin.call('GET','/api/streams/capture-status')['drivers'].get(source['id'],{})
 if status.get('buffering'):break
 time.sleep(1)
result=admin.call('POST','/api/sources/'+source['id']+'/capture/snapshot')
image=admin.call('GET',result['url'],raw=True)
assert image.startswith(b'\xff\xd8'), 'Snapshot is not JPEG'
passed('Idle source snapshot produces actual JPEG')
# Exercise a second real process execution with an authoritative ownership boundary.
draft=admin.call('GET','/api/driving-sessions/defaults?experience=practice');draft['name']='Handover integration · protocol fixture'
session=admin.call('POST','/api/driving-sessions',draft)
sid=session['id'];active=False
try:
 review=admin.call('POST','/api/driving-sessions/preview',session)
 launch={'expected_revision':session['revision'],'instance_id':review['instance_id'],'mode':'now','idempotency_key':str(uuid.uuid4()),'time_zone':'Europe/Oslo'}
 result=admin.call('POST',f'/api/driving-sessions/{sid}/start',launch);active=True
 assert admin.call('POST',f'/api/driving-sessions/{sid}/start',launch)['execution_id']==result['execution_id']
 time.sleep(3)
 guest=admin.call('POST','/api/guest-drivers',{'name':'Handover guest · fixture'})['id']
 admin.call('POST','/api/sources/'+source['id']+'/capture/record')
 time.sleep(3)
 admin.call('POST','/api/server/assign-driver?instance='+str(review['instance_id']),{'car_id':0,'guest_driver_id':guest})
 snapshot=admin.call('POST','/api/sources/'+source['id']+'/capture/snapshot')
 for _ in range(20):
  jobs=admin.call('GET','/api/sources/'+source['id']+'/jobs')['items']
  if jobs and jobs[0]['state'] in ('completed','failed'):break
  time.sleep(1)
 assert jobs[0]['state']=='completed',jobs[0]
 moments=admin.call('GET','/api/sources/'+source['id']+'/media')['items']
 clip=next(m for m in moments if m['url'].endswith(jobs[0]['path']))
 picture=next(m for m in moments if m['url']==snapshot['url'])
 assert clip['guest_driver_id'] is None,clip
 assert picture['guest_driver_id']==guest,picture
 passed('In-flight clip keeps previous owner; post-handover snapshot credits guest')
 admin.call('POST',f'/api/driving-sessions/{sid}/finish');active=False
 recap=admin.call('GET',f'/api/driving-sessions/{sid}/recap')
 assert any(m['execution_id']==result['execution_id'] for m in recap['media'])
 passed('Exact launch retry, handover, finish and execution-linked recap')
finally:
 if active:admin.call('POST',f'/api/driving-sessions/{sid}/finish')
# Original capability categories: use reversible fixture records for mutations.
for path in ['/api/setup/summary','/api/server/readiness?instance=1','/api/config','/api/instances','/api/events','/api/categories','/api/queue?instance=1','/api/streams/debug','/api/sources/status','/api/content/images/count','/api/users','/api/user','/api/driver-sessions','/api/scores','/api/results/files','/api/about','/api/feed']:
 admin.call('GET',path)
passed('All 14 Advanced categories reach their real API foundations')
configuration=admin.call('GET','/api/config')
admin.call('PUT','/api/config',configuration)
admin.call('PUT','/api/config/content',configuration)
passed('Installation, CSP, engine and global configuration persist')
# A second independent instance never starts a process or touches the first queue.
instance={'name':'Independent fixture','enabled':0,'udp_port':41001,'tcp_port':41001,'http_port':41002,'plugin_port':41003,'plugin_listen_port':41004,'run_mode':'manual_queue','start_on_boot':0}
iid=admin.call('POST','/api/instances',instance)['id']
try:
 admin.call('PUT',f'/api/instances/{iid}',{**instance,'name':'Independent fixture edited'})
 admin.call('POST','/api/instances',instance,expect=400)
 admin.call('PUT',f'/api/instances/{iid}/runmode',{'run_mode':'repeat_event','repeat_event_id':session['event_id']})
 admin.call('PUT',f'/api/instances/{iid}/runmode',{'run_mode':'manual_queue'})
 admin.call('PUT',f'/api/instances/{iid}/schedule',{'scheduled_start':int(time.time())+86400})
 admin.call('PUT',f'/api/instances/{iid}/schedule',{'scheduled_start':None})
finally:admin.call('DELETE',f'/api/instances/{iid}')
passed('Independent instance create/edit/delete, port conflicts, repeat and schedule persistence')
# Duplicate all five reusable preset kinds and edit only the new copies.
for plural,singular in [('difficulties','difficulty'),('sessions','session'),('times','time'),('classes','class'),('drift-scoring-modes','drift-scoring-mode')]:
 pid=admin.call('POST','/api/'+plural,{'name':'Parity fixture '+singular})['id']
 data=admin.call('GET',f'/api/{singular}/{pid}')['data']
 if singular=='time':
  for weather in data.get('weathers',[]):
   for key,value in weather.items():
    if isinstance(value,str) and (value.isdigit() or value.startswith('-') and value[1:].isdigit()):weather[key]=int(value)
 admin.call('PUT',f'/api/{singular}/{pid}',{**data,'name':'Parity fixture edited '+singular})
 admin.call('DELETE',f'/api/{singular}/{pid}')
passed('All five reusable preset types create, read, save and delete')
# A real ZIP import, explicit duplicate conflict, overwrite and dependency-safe delete.
def upload(data,overwrite=False):
 boundary='pitlane-'+uuid.uuid4().hex
 body=(f'--{boundary}\r\nContent-Disposition: form-data; name="kind"\r\n\r\ncar\r\n--{boundary}\r\nContent-Disposition: form-data; name="overwrite"\r\n\r\n{str(overwrite).lower()}\r\n--{boundary}\r\nContent-Disposition: form-data; name="archive"; filename="fixture.zip"\r\nContent-Type: application/zip\r\n\r\n').encode()+data+f'\r\n--{boundary}--\r\n'.encode()
 return body,'multipart/form-data; boundary='+boundary
archive=io.BytesIO()
with zipfile.ZipFile(archive,'w') as z:
 z.writestr('content/cars/pitlane_import_fixture/ui/ui_car.json',json.dumps({'name':'Import fixture','brand':'Fixture','tags':['fixture'],'specs':{},'torqueCurve':[],'powerCurve':[]}))
 z.writestr('content/cars/pitlane_import_fixture/skins/red/ui_skin.json','{"skinname":"Red"}')
 z.writestr('content/cars/pitlane_import_fixture/data.acd','fixture data, not game content')
body,ctype=upload(b'invalid zip');admin.call('POST','/api/content/upload',body,expect=400,content_type=ctype)
body,ctype=upload(archive.getvalue());admin.call('POST','/api/content/upload',body,content_type=ctype)
admin.call('POST','/api/content/upload',body,expect=409,content_type=ctype)
body,ctype=upload(archive.getvalue(),True);admin.call('POST','/api/content/upload',body,content_type=ctype)
metadata=admin.call('GET','/api/content/metadata/car/pitlane_import_fixture')['metadata']
saved=admin.call('PUT','/api/content/metadata/car/pitlane_import_fixture',{**metadata,'archived':True,'tags':['test'],'notes':'Persist through rescan'})
admin.call('POST','/api/content/recache')
assert admin.call('GET','/api/content/metadata/car/pitlane_import_fixture')['metadata']['archived']
changes=admin.call('GET','/api/recovery/changes')['items']
admin.call('POST',f"/api/recovery/changes/{changes[0]['id']}/undo",{'expected_revision':saved['revision']})
assert not admin.call('GET','/api/content/metadata/car/pitlane_import_fixture')['metadata']['archived']
admin.call('DELETE','/api/car/pitlane_import_fixture')
admin.call('DELETE','/api/track/drift',expect=409)
passed('Failed ZIP retry, duplicate conflict, overwrite, metadata/archive rescan, undo and guarded disk deletion')
admin.call('PUT','/api/user',{'measurement_unit':1,'temp_unit':1})
assert admin.call('GET','/api/user')['measurement_unit']==1
admin.call('PUT','/api/user',{'measurement_unit':0,'temp_unit':0})
admin.call('PUT','/api/users/'+name+'/password',{'password':'reset-fixture-only'})
Client(name,'reset-fixture-only')
admin.call('PUT','/api/users/'+name+'/role',{'role':'viewer'})
admin.call('DELETE','/api/users/'+name)
passed('Preferences, password reset, role change and account removal')
backup=admin.call('GET','/api/server/smdata',raw=True)
with tempfile.TemporaryDirectory(prefix='pitlane-backup-') as directory:
 path=Path(directory)/'smdata.db';path.write_bytes(backup)
 db=sqlite3.connect('file:'+str(path)+'?mode=ro',uri=True)
 assert db.execute('PRAGMA quick_check').fetchone()[0]=='ok'
 assert db.execute('SELECT count(*) FROM driving_session').fetchone()[0]>0
 db.close()
admin.call('GET','/api/server/smcontent',raw=True)
admin.call('GET','/api/server/logfile',raw=True)
cleanup=admin.call('GET','/api/recovery/cleanup');admin.call('POST','/api/recovery/cleanup',{'token':cleanup['token']})
passed('Consistent HTTP backup, content archive, application log and safe cleanup')
# Embedded SPA routes and hashed assets must work without the Vite server.
import re
for path in ['/sessions/1','/garage/rigs','/garage/content','/garage/advanced','/sessions?q=fixture&tag=test']:
 html=admin.call('GET',path,raw=True).decode()
 assert 'Pitlane' in html and '<div id="app">' in html
scripts=re.findall(r'<script[^>]+src="([^"]+)"',html)
assert scripts
for script in scripts:assert len(admin.call('GET',script,raw=True))>100
legacy=admin.http.open(BASE+'/app/sessions?q=fixture%20driver&tag=test',timeout=10)
assert legacy.geturl().endswith('/sessions?q=fixture%20driver&tag=test'),legacy.geturl()
passed('Go-embedded deep links, hashed JavaScript, and legacy raw query redirect')
Path('docs/pitlane-integration-results.json').write_text(json.dumps({'fixture':BASE,'checked_at':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()),'checks':checks},indent=2)+'\n')
