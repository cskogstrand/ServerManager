// One source catalogue/health poll shared by the watch surfaces. Game presence
// supplies attribution and telemetry, never permission to play a camera.
import { ref } from 'vue';
import { api } from '@/lib/api';
import type { DriverState } from '@/stores/server';
import type { DriverStream } from '@/types/generated';
export type StreamHealthStatus = 'live' | 'offline' | 'unknown' | 'not_configured';
export interface StreamHealth { status:StreamHealthStatus; status_code?:number; message?:string; checked_at?:number }
export interface CameraSource {
  id:string; version?:string; name:string; enabled:boolean; player_url:string; status_url?:string; capture_url?:string;
  recording_configured:boolean; driver_guid:string; instance_id:number|null; rig_id:number|null;
}
export interface StreamChannel {
  key:string; title:string; subtitle?:string; url:string; kind?:'iframe'|'whep';
  health:StreamHealthStatus; online:boolean; driverGuid?:string; instanceId?:number|null;
}
export function isWhepUrl(url:string|undefined|null):boolean {
  if(!url)return false;
  try{return new URL(url).pathname.replace(/\/+$/,'').endsWith('/whep')}catch{return /\/whep(\?|#|$)/.test(url)}
}
const sources=ref<CameraSource[]>([]), health=ref<Record<string,StreamHealth>>({}), byGuid=ref<Record<string,DriverStream>>({});
const loaded=ref(false), error=ref('');
let generation=0;
let loading:Promise<void>|null=null, checking:Promise<void>|null=null;
const subscribers=new Set<symbol>(); let timer:ReturnType<typeof setInterval>|null=null;
async function loadStreams(){
  if(loading)return loading;
  const current=generation;
  loading=(async()=>{try{
    const result=await api.get<{sources:CameraSource[]}>('/api/sources');
    if(current!==generation)return;
    sources.value=result.sources??[];
    byGuid.value=Object.fromEntries(sources.value.filter(s=>s.enabled&&s.driver_guid).map(s=>[s.driver_guid,{driver_guid:s.driver_guid,display_name:s.name,enabled:1,stream_embed_url:s.player_url}]));
    error.value='';loaded.value=true;
  }catch(e){if(current===generation)error.value=e instanceof Error?e.message:'Could not load cameras';}finally{if(current===generation)loading=null}})();
  return loading;
}
async function refreshHealth(_instanceId?:number){
  if(checking)return checking;
  const current=generation;
  checking=(async()=>{try{const result=await api.get<{statuses:Record<string,StreamHealth>}>('/api/sources/status');if(current===generation)health.value=result.statuses??{}}
    catch{if(current===generation)health.value=Object.fromEntries(Object.entries(health.value).map(([id,h])=>[id,{...h,status:'unknown',message:'Status update unavailable'}]));}
    finally{if(current===generation)checking=null}})();return checking;
}
export function clearDriverStreams(){generation++;loading=null;checking=null;subscribers.clear();if(timer)clearInterval(timer);timer=null;sources.value=[];byGuid.value={};health.value={};loaded.value=false;error.value="";}
export function useDriverStreams(){
  const subscriber=Symbol();
  function startHealthPoll(_instanceId?:()=>number,_intervalMs=15000){
    subscribers.add(subscriber);void refreshHealth();
    if(!timer)timer=setInterval(()=>{if(document.visibilityState!=='hidden')void refreshHealth()},15000);
  }
  function stopHealthPoll(){subscribers.delete(subscriber);if(!subscribers.size&&timer){clearInterval(timer);timer=null}}
  function streamForGuid(guid:string|null|undefined):DriverStream|null{return guid?byGuid.value[guid]??null:null}
  function healthForGuid(guid:string|null|undefined):StreamHealthStatus{const s=sources.value.find(s=>s.driver_guid===guid&&s.enabled);return s?health.value[s.id]?.status??'unknown':'not_configured'}
  function allChannelsFor(drivers:DriverState[]):StreamChannel[]{
    return sources.value.filter(s=>s.enabled&&s.player_url).map(s=>{
      const driver=drivers.find(d=>d.guid===s.driver_guid&&d.connected);
      return {key:s.id,title:s.name,subtitle:driver?`Driving: ${driver.name}`:s.driver_guid?'No game connection':'Standalone camera',url:s.player_url,
        kind:isWhepUrl(s.player_url)?'whep':'iframe',health:health.value[s.id]?.status??'unknown',
        // Even an unavailable status endpoint may accompany playable video.
        online:true,driverGuid:s.driver_guid,instanceId:s.instance_id} satisfies StreamChannel;
    });
  }
  function keyForGuid(guid:string|undefined){return sources.value.find(s=>s.driver_guid===guid&&s.enabled)?.id??null}
  return {sources,byGuid,health,loaded,error,loadStreams,refreshHealth,startHealthPoll,stopHealthPoll,streamForGuid,healthForGuid,allChannelsFor,keyForGuid};
}
