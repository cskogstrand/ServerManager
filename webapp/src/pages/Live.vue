<script setup lang="ts">
import { computed,onMounted,onBeforeUnmount,ref } from 'vue';
import { useDriverStreams } from '@/lib/useDriverStreams';
import { useServerStore } from '@/stores/server';
import { useAuthStore } from '@/stores/auth';
import SourcePlayer from '@/components/SourcePlayer.vue';
import StreamTheater from '@/components/StreamTheater.vue';
const streams=useDriverStreams(),server=useServerStore(),auth=useAuthStore();
const filter=ref('all'),theater=ref(false),key=ref<string|null>(null);
const channels=computed(()=>streams.allChannelsFor(server.instanceList.flatMap(i=>i.drivers)));
const shown=computed(()=>channels.value.filter(c=>filter.value==='all'||filter.value==='live'&&streams.health.value[c.key]?.status==='live'||c.instanceId===Number(filter.value)||server.instances[Number(filter.value)]?.drivers.some(d=>d.guid===c.driverGuid&&d.connected)));
const playback=ref<Record<string,string>>({});
onMounted(()=>{void streams.loadStreams();streams.startHealthPoll()});onBeforeUnmount(streams.stopHealthPoll);
function watchSource(id:string){key.value=id;theater.value=true}
</script>
<template>
<header class="pitlane-heading"><div><span class="pitlane-eyebrow">Around the club</span><h1>Live<span class="text-accent">.</span></h1><p>Every connected camera. A drive in progress, or just a seat waiting for its next driver.</p></div><RouterLink v-if="auth.isAdmin" to="/garage/rigs?cameras=1" class="pitlane-button">Set up streams →</RouterLink></header>
<div class="pitlane-tabs"><button :aria-pressed="filter==='all'" @click="filter='all'">All sources</button><button :aria-pressed="filter==='live'" @click="filter='live'">Status confirmed</button><button v-for="i in server.instanceList" :key="i.id" :aria-pressed="filter===String(i.id)" @click="filter=String(i.id)">{{ i.name }}</button></div>
<div v-if="streams.error.value" role="alert" class="pitlane-error">{{ streams.error.value }} <button @click="streams.loadStreams">Retry</button></div>
<div v-if="!streams.loaded.value && !streams.error.value" role="status">Loading cameras…</div>
<div v-else-if="!shown.length" class="pitlane-empty"><h2 class="text-2xl">{{ channels.length?'No cameras match this view.':'Bring your club into view.' }}</h2><p>{{ channels.length?'Try all sources to check their status.':'Add a simulator or spectator camera. A game connection is optional.' }}</p><RouterLink v-if="auth.isAdmin" to="/garage/rigs?cameras=1" class="pitlane-button primary">Set up streams</RouterLink></div>
<div class="pitlane-grid"><article v-for="channel in shown" :key="channel.key" class="live-camera"><div class="live-video"><SourcePlayer v-if="!theater" :url="channel.url" :name="channel.title" @playing="playback[channel.key]='Playing'" @error="playback[channel.key]=$event" /></div><div class="p-5"><h2 class="text-xl">{{ channel.title }}</h2><p class="text-muted text-sm mt-2">{{ channel.subtitle }}</p><p class="text-muted text-sm mt-2">{{ playback[channel.key] || (channel.kind==='iframe'?'Embedded player · status not observable':'Playback not confirmed') }}</p><p class="text-xs text-muted mt-1">Status endpoint: {{ channel.health === 'live' ? 'Responding' : channel.health }} · recording checked separately</p><div class="pitlane-actions mt-4"><button class="pitlane-button" @click="watchSource(channel.key)">Enlarge</button><RouterLink :to="`/garage/cameras/${encodeURIComponent(channel.key)}`" class="pitlane-button">Camera & moments</RouterLink><RouterLink v-if="auth.isAdmin" :to="`/garage/rigs?source=${encodeURIComponent(channel.key)}`" class="pitlane-button">Stream setup</RouterLink></div></div></article></div>
<section v-if="server.instanceList.some(i=>i.running)" class="pitlane-section"><h2>On track now</h2><div class="pitlane-actions"><RouterLink v-for="i in server.instanceList.filter(i=>i.running)" :key="i.id" :to="`/server/${i.id}/broadcast`" class="pitlane-button">Watch {{ i.name }} →</RouterLink></div></section>
<StreamTheater :open="theater" :channels="channels" :initial-key="key" @close="theater=false" />
</template>
<style scoped>.live-camera{border:1px solid var(--color-line);border-radius:14px;overflow:hidden;background:var(--color-surface)}.live-video{aspect-ratio:16/10;background:#111813}</style>
