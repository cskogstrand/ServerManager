<script setup lang="ts">
import {computed,onMounted,onBeforeUnmount,ref} from 'vue';
import {useRoute} from 'vue-router';
import {api} from '@/lib/api';
import {useDriverStreams} from '@/lib/useDriverStreams';
import {useDriverCapture} from '@/lib/useDriverCapture';
import {useAuthStore} from '@/stores/auth';
import ManualRecordings from '@/components/ManualRecordings.vue';
import type {MediaItem} from '@/types/driverStats';
import {listGuestDrivers,type GuestDriver} from '@/lib/guestDriversApi';
import {useConfirmStore} from '@/stores/confirm';
import SourcePlayer from '@/components/SourcePlayer.vue';
const route=useRoute(),streams=useDriverStreams(),cap=useDriverCapture(),auth=useAuthStore();
const confirm=useConfirmStore(), guests=ref<GuestDriver[]>([]), deleting=ref(new Set<string>());
const source=computed(()=>streams.sources.value.find(s=>s.id===route.params.key));
const target=computed(()=>source.value?.id.startsWith('source:')?source.value.id:source.value?.driver_guid??'');
const media=ref<MediaItem[]>([]);
const jobs=ref<{id:number;state:string;requested_at:number;message:string}[]>([]),error=ref(''),playback=ref('Playback not confirmed'),busy=ref(false);
let timer:ReturnType<typeof setInterval>|undefined;
async function refresh(){try{const base=`/api/sources/${encodeURIComponent(String(route.params.key))}`;const [moments,requests]=await Promise.all([api.get<{items:typeof media.value}>(`${base}/media`),api.get<{items:typeof jobs.value}>(`${base}/jobs`)]);media.value=moments.items.map(m=>({...m,id:String(m.id)}));error.value="";jobs.value=requests.items}catch(e){error.value=String(e)}}
async function capture(action:string){busy.value=true;error.value='';try{if(action==='snapshot')await cap.takePicture(target.value);else if(cap.isRecording(target.value))await cap.stopRecording(target.value);else await cap.recordNow(target.value);await refresh()}catch(e){error.value=String(e)}finally{busy.value=false}}
async function removeMedia(m:MediaItem){
 if(!await confirm.ask({title:'Delete saved moment?',message:'Permanently delete this recording or snapshot?',confirmLabel:'Delete moment',tone:'danger'}))return;
 deleting.value.add(m.id);try{await api.delete(m.url);await refresh()}catch(e){error.value=String(e)}finally{deleting.value.delete(m.id)}
}
async function assignMedia(m:MediaItem, guest:number|null){try{await api.post(`${m.url}/assign`,{guest_driver_id:guest});await refresh()}catch(e){error.value=String(e)}}
function download(m:MediaItem){const a=document.createElement('a');a.href=`${m.url}?download=1`;a.download='';a.click()}
onMounted(async()=>{try{guests.value=await listGuestDrivers()}catch(e){error.value=String(e)}await streams.loadStreams();streams.startHealthPoll();cap.startPoll();await refresh();timer=setInterval(refresh,5000)});onBeforeUnmount(()=>{streams.stopHealthPoll();cap.stopPoll();clearInterval(timer)});
</script>
<template><header class="pitlane-heading"><div><span class="pitlane-eyebrow">Live / Camera</span><h1>{{ source?.name||'Camera' }}</h1><p>{{ source?.driver_guid?'Linked game account · attribution follows the current driver':'Standalone source · no driver attribution' }}</p></div><div class="pitlane-actions"><RouterLink to="/live" class="pitlane-button">All cameras</RouterLink><RouterLink v-if="auth.isAdmin&&source" :to="`/garage/rigs?source=${encodeURIComponent(source.id)}`" class="pitlane-button">Stream setup</RouterLink></div></header><div v-if="error" class="pitlane-error" role="alert">{{ error }}</div><div v-if="source" class="camera-detail-grid"><section class="pitlane-card"><div class="aspect-video"><SourcePlayer :url="source.player_url" :name="source.name" selected @playing="playback='Playing'" @error="playback=$event"/></div><p>{{ playback }}</p><p>Status endpoint: {{ streams.health.value[source.id]?.status??'unknown' }}</p><p>Recorder: {{ cap.isBuffering(target)?'Receiving video segments':'No fresh buffered video' }}</p><div v-if="auth.canOperate&&target" class="pitlane-actions mt-4"><button class="pitlane-button" :disabled="busy||!cap.isBuffering(target)" @click="capture('snapshot')">Snapshot</button><button class="pitlane-button" :disabled="busy||(!cap.isBuffering(target)&&!cap.isRecording(target))" @click="capture('record')">{{ cap.isRecording(target)?'Stop & save recording':'Record clip' }}</button></div></section><section class="pitlane-card"><h2>Recording requests</h2><p v-if="!jobs.length">No manual recordings requested for this camera.</p><div v-for="job in jobs" :key="job.id" class="py-4 border-b border-line"><b>{{ job.state }}</b><p>{{ job.message }}</p><small>{{ new Date(job.requested_at).toLocaleString() }}</small></div></section></div><section class="pitlane-section"><p v-if="!media.length">No saved moments for this source.</p><ManualRecordings :items="media" heading="Saved moments" context="Captured by this camera" :initial-file="typeof route.query.media==='string'?route.query.media:undefined" :account-label="source?.driver_guid?'Account driver':'Unassigned source'" :can-operate="auth.canOperate" :guests="guests" :deleting-ids="deleting" @delete-media="removeMedia" @assign="assignMedia" @download-media="download"/></section></template>
<style scoped>.camera-detail-grid{display:grid;grid-template-columns:2fr 1fr;gap:24px}@media(max-width:800px){.camera-detail-grid{grid-template-columns:1fr}}</style>
