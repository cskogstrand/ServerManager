<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useServerStore } from '@/stores/server';
import { useAuthStore } from '@/stores/auth';
import { useSetupSummary } from '@/lib/useSetupSummary';
import {api} from '@/lib/api';
import type {DrivingSession} from '@/lib/drivingSessions';
import TrackImage from '@/components/TrackImage.vue';
import LiveFeed from '@/components/LiveFeed.vue';
const server = useServerStore();
const auth = useAuthStore();
const { summary, error, reload } = useSetupSummary();
const setupBlocker=computed(()=>summary.value?.blocking?.find(b=>['install','content','server','instance'].includes(b.step)));
const running = computed(() => server.instanceList.filter(i => i.running));
const sessions=ref<DrivingSession[]>([]),scheduleError=ref('');
const nextSessions=computed(()=>sessions.value.filter(s=>['planned','queued'].includes(s.lifecycle)).sort((a,b)=>(a.scheduled_at??0)-(b.scheduled_at??0)));
async function loadSchedules(){try{sessions.value=(await api.get<{sessions:DrivingSession[]}>('/api/driving-sessions')).sessions;scheduleError.value=''}catch{scheduleError.value='Upcoming sessions could not be loaded.'}}
const planned = computed(() => server.instanceList.filter(i => i.scheduled_start).sort((a,b) => (a.scheduled_start ?? 0) - (b.scheduled_start ?? 0)));
const date = new Intl.DateTimeFormat(undefined, { weekday:'long', day:'numeric', month:'long' }).format(new Date());
const when = (time: number) => new Date(time * 1000).toLocaleString(undefined, {dateStyle:'medium',timeStyle:'short'});
onMounted(()=>{void reload();void loadSchedules()});
</script>
<template>
  <header class="pitlane-heading"><div><span class="pitlane-eyebrow">{{ date }}</span><h1>Today, we drive<span class="text-accent">.</span></h1><p>A little less organizing. A lot more time on track.</p></div><div class="pitlane-actions"><RouterLink class="pitlane-button" to="/live">Live streams</RouterLink><RouterLink v-if="auth.canOperate" class="pitlane-button primary" to="/sessions/new">New session →</RouterLink></div></header>
  <div v-if="error" role="alert" class="pitlane-error">Readiness unavailable. {{ error }} <button class="underline" @click="reload">Retry</button></div>
  <div class="today-layout">
    <section class="pitlane-stack">
      <article v-for="instance in running" :key="instance.id" class="today-hero">
        <TrackImage v-if="instance.session?.track" variant="photo" :track-key="instance.session.track" :config="instance.session.track_config" class="today-photo" />
        <div class="today-hero-copy"><span class="pitlane-eyebrow">On track now · {{ instance.name }}</span><h2>{{ instance.session?.name || instance.name }}</h2><p>{{ instance.session?.track || 'Waiting for track details' }} · {{ instance.players }} drivers</p><RouterLink :to="`/server/${instance.id}`" class="pitlane-button">Open session →</RouterLink></div>
      </article>
      <article v-if="!running.length" class="today-welcome pitlane-card"><span class="pitlane-eyebrow">Make time for a drive</span><h2>{{ server.loaded ? 'The track is waiting.' : 'Checking your club…' }}</h2><p v-if="server.loaded">{{ setupBlocker ? setupBlocker.message : 'Drift, race, or take a few practice laps. Start with the experience.' }}</p><RouterLink v-if="auth.isAdmin && setupBlocker" to="/setup" class="pitlane-button primary">Finish setup →</RouterLink><RouterLink v-else-if="auth.canOperate" to="/sessions/new" class="pitlane-button primary">Prepare a session →</RouterLink><p v-else>Your host can prepare the next session. Watch cameras in Live while you wait.</p></article>
    </section>
    <aside class="pitlane-card"><span class="pitlane-eyebrow">Up next</span><p v-if="scheduleError" role="alert">{{ scheduleError }} <button class="underline" @click="loadSchedules">Retry</button></p><div v-for="session in nextSessions" :key="session.id" class="mb-6"><h2>{{ session.name }}</h2><p>{{ session.scheduled_at ? new Date(session.scheduled_at).toLocaleString() : 'After the current running order' }}</p><RouterLink :to="`/sessions/${session.id}`" class="pitlane-button">Review session →</RouterLink></div><template v-if="planned.length"><div v-for="instance in planned" :key="instance.id" class="mb-6"><h2>{{ instance.name }}</h2><p>{{ when(instance.scheduled_start!) }}</p><RouterLink :to="`/queue?instance=${instance.id}`" class="pitlane-button">Review running order →</RouterLink></div></template><template v-else-if="!nextSessions.length&&!scheduleError"><h2>An open evening.</h2><p>No scheduled starts. Prepare the next session when you’re ready.</p><RouterLink to="/sessions" class="pitlane-button">See sessions →</RouterLink></template></aside>
  </div>
  <div class="today-health"><span>{{ running.length }} {{ running.length === 1 ? 'server' : 'servers' }} running</span><span>{{ server.instanceList.length }} servers in your club</span><RouterLink to="/garage">Your servers →</RouterLink></div>
  <section class="pitlane-section"><LiveFeed /></section>
</template>
<style scoped>
.today-layout{display:grid;grid-template-columns:minmax(0,1.9fr) minmax(260px,1fr);gap:24px}.today-welcome{min-height:356px;display:flex;flex-direction:column;align-items:start;justify-content:center;background:var(--color-surface-2)}.today-welcome h2{font-size:38px}.today-welcome p{max-width:430px}.today-hero{position:relative;isolation:isolate;min-height:356px;display:flex;align-items:end;border-radius:16px;overflow:hidden;background:#2e3b31;color:#fff}.today-photo{position:absolute!important;inset:0;width:100%;height:100%;z-index:-2}.today-hero:after{content:'';position:absolute;inset:0;background:linear-gradient(180deg,#07140d20,#0a1e13eb);z-index:-1}.today-hero-copy{padding:30px;width:100%}.today-hero-copy .pitlane-eyebrow{color:#e4e9e0}.today-hero h2{font-size:38px;letter-spacing:-.05em}.today-hero p{margin:12px 0 22px}.today-hero .pitlane-button{color:#273126;background:#fff}.today-health{display:flex;flex-wrap:wrap;gap:24px;font-size:13px;color:var(--color-muted);padding:20px 6px}.today-health a{margin-left:auto}@media(max-width:767px){.today-layout{grid-template-columns:1fr}.today-welcome,.today-hero{min-height:310px}}
</style>
