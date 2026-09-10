<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "@/lib/api";
import { useServerStore, type InstanceState } from "@/stores/server";
import { useContentStore } from "@/stores/content";
import { useSetupSummary } from "@/lib/useSetupSummary";
import { useDriverStreams, type StreamChannel } from "@/lib/useDriverStreams";
import { useAuthStore } from "@/stores/auth";
import PageHeader from "@/components/ui/PageHeader.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import Skeleton from "@/components/ui/Skeleton.vue";
import TrackImage from "@/components/TrackImage.vue";
import StreamWall from "@/components/StreamWall.vue";
import StreamTheater from "@/components/StreamTheater.vue";
import LiveFeed from "@/components/LiveFeed.vue";

interface StatusPayload {
  current_event: { id: number; name: string; track: string; track_key: string; track_config: string; class: string; session: string; time: string; weather: string };
}
const route = useRoute();
const server = useServerStore();
const content = useContentStore();
const auth = useAuthStore();
const { summary, error: readinessError, reload: reloadSummary } = useSetupSummary();
const details = ref<Record<number, StatusPayload>>({});
const detailErrors = ref<Record<number, string>>({});
const actionErrors = ref<Record<number, string>>({});
const busy = ref<Record<number, boolean>>({});
const loading = ref(true);
const loadError = ref("");
const driverStreams = useDriverStreams();
const theaterOpen = ref(false);
const theaterKey = ref<string | null>(null);
const showOfflineStreams = ref(false);
const channels = computed<StreamChannel[]>(() => driverStreams.allChannelsFor(server.instanceList.flatMap(i => i.drivers)));
const onlineChannels = computed(() => channels.value.filter(channel => channel.online));
const runningCount = computed(() => server.instanceList.filter(instance => instance.running).length);
const driversCount = computed(() => server.instanceList.reduce((total, instance) => total + instance.players, 0));
const pendingCount = (id: number) => summary.value?.instances.find(instance => instance.id === id)?.queue_pending ?? 0;
const startable = (instance: InstanceState) => !readinessError.value && !detailErrors.value[instance.id] && !server.statusErrors[instance.id] && !!summary.value?.can_start && (instance.run_mode === "repeat_event" ? !!instance.repeat_event_id : pendingCount(instance.id) > 0);
const controlTo = (id: number) => `/server/${id}`;
const queueTo = (id: number) => ({ name: "queue", query: { instance: id } });
function elapsed(instance: InstanceState) {
  const seconds = Math.max(0, Math.floor((instance.session?.elapsed_ms ?? 0) / 1000));
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}
function status(instance: InstanceState) {
  if (detailErrors.value[instance.id] || server.statusErrors[instance.id]) return "Status unavailable";
  return instance.running ? "Running" : "Stopped";
}
async function fetchDetail(id: number) {
  try { details.value[id] = await api.get<StatusPayload>(`/api/server/status?instance=${id}`); delete detailErrors.value[id]; }
  catch (error) { detailErrors.value[id] = error instanceof Error ? error.message : "Could not load race details."; }
}
async function refreshDetails() { await Promise.all([reloadSummary(), ...server.instanceList.map(instance => fetchDetail(instance.id))]); }
async function loadDashboard() {
  loadError.value = "";
  loading.value = true;
  try { await Promise.all([server.load(), content.load()]); await refreshDetails(); }
  catch (error) { loadError.value = error instanceof Error ? error.message : "Could not load the overview."; }
  finally { loading.value = false; }
}
async function start(instance: InstanceState) {
  if (!startable(instance) || busy.value[instance.id]) return;
  busy.value[instance.id] = true;
  delete actionErrors.value[instance.id];
  try { await server.start(instance.id); await fetchDetail(instance.id); }
  catch (error) { actionErrors.value[instance.id] = error instanceof Error ? error.message : "Could not start the server. Try again."; }
  finally { busy.value[instance.id] = false; }
}
function watchStream(key?: string) { theaterKey.value = key ?? null; theaterOpen.value = true; }
watch(() => server.instanceList.map(instance => `${instance.id}:${instance.running}`).join(","), refreshDetails);
onMounted(() => { void loadDashboard(); void driverStreams.loadStreams(); });
</script>

<template>
  <PageHeader title="Your servers" subtitle="See what’s running, prepare the next race, and resolve anything that needs attention." icon="dashboard">
    <template #actions>
      <RouterLink v-if="auth.canOperate" to="/events" class="inline-flex min-h-11 items-center gap-2 rounded-md bg-primary px-4 text-sm font-semibold text-white hover:brightness-110"><Icon name="plus" :size="17" />Prepare a race</RouterLink>
    </template>
  </PageHeader>
  <p v-if="route.query.choose === 'server'" role="status" class="mb-5 rounded-md border border-accent/40 bg-accent-dim px-4 py-3 text-sm">Choose a server below to open its Race Control.</p>
  <div v-if="loadError" role="alert" class="mb-5 rounded-md border border-danger/40 bg-danger-glow p-4 text-sm"><p class="font-semibold text-danger">Could not refresh the overview</p><p class="mt-1 text-muted">{{ loadError }}</p><Button class="mt-3" variant="dark" @click="loadDashboard">Retry</Button></div>
  <div v-if="loading && !server.instanceList.length" class="grid gap-4 lg:grid-cols-2"><Skeleton v-for="n in 2" :key="n" class="h-72" /></div>
  <EmptyState v-else-if="!server.instanceList.length && !loadError" icon="instances" title="No servers yet" :message="auth.isAdmin ? 'Add a server instance, then choose a race setup to run.' : 'An administrator needs to add a server before races can run.'">
    <RouterLink v-if="auth.isAdmin" to="/settings/instances" class="inline-flex min-h-11 items-center rounded-md bg-primary px-4 text-sm font-semibold text-white">Add a server</RouterLink>
  </EmptyState>
  <template v-else-if="server.instanceList.length">
    <div class="mb-6 flex flex-wrap gap-x-6 gap-y-2 text-sm text-muted"><span><strong class="text-text">{{ server.instanceList.length }}</strong> servers</span><span><strong class="text-text">{{ runningCount }}</strong> running</span><span><strong class="text-text">{{ driversCount }}</strong> drivers</span><span class="ml-auto">{{ server.connected ? 'Live connection' : 'Last known state' }}</span></div>
    <div class="grid items-start gap-5 lg:grid-cols-2">
      <article v-for="instance in server.instanceList" :key="instance.id" class="overflow-hidden rounded-lg border border-line bg-surface shadow-sm">
        <header class="flex flex-wrap items-center justify-between gap-3 px-5 pt-5"><h2 class="text-base font-semibold"><RouterLink :to="controlTo(instance.id)" class="hover:text-accent">{{ instance.name }}</RouterLink></h2><span class="inline-flex items-center gap-2 rounded-md px-2.5 py-1 text-xs font-semibold" :class="detailErrors[instance.id] || server.statusErrors[instance.id] ? 'bg-warn-glow text-warn' : instance.running ? 'bg-ok-glow text-ok' : 'bg-surface-2 text-muted'"><Icon :name="instance.running ? 'activity' : 'stop'" :size="13" />{{ status(instance) }}</span></header>
        <div class="p-5">
          <p v-if="detailErrors[instance.id]" role="alert" class="mb-4 text-sm text-warn">{{ detailErrors[instance.id] }} <button type="button" class="min-h-11 text-accent underline" @click="fetchDetail(instance.id)">Retry details</button></p>
          <template v-if="details[instance.id]?.current_event?.id">
            <TrackImage :track-key="details[instance.id].current_event.track_key" :config="details[instance.id].current_event.track_config" class="mb-4 h-36 w-full rounded-md border border-line" />
            <p class="mb-1 text-xs font-medium text-muted">{{ instance.running ? 'On track' : 'Last loaded race' }}</p>
            <h3 class="text-xl font-semibold tracking-tight">{{ details[instance.id].current_event.name || details[instance.id].current_event.track }}</h3>
            <p class="mt-2 text-sm text-muted">{{ details[instance.id].current_event.class }} · {{ details[instance.id].current_event.session }} · {{ details[instance.id].current_event.time }}</p>
          </template>
          <div v-else class="mb-4 rounded-md border border-line bg-surface-2/50 p-5"><Icon name="events" :size="24" class="mb-3 text-accent" /><h3 class="text-xl font-semibold">{{ startable(instance) ? 'Ready for the next race.' : 'Prepare your next race.' }}</h3><p class="mt-2 text-sm text-muted">{{ instance.running ? 'Waiting for race details.' : startable(instance) ? 'Review the run plan when you are ready to begin.' : 'Choose a saved setup and review it before starting.' }}</p></div>
          <dl v-if="instance.running" class="mt-5 grid grid-cols-2 gap-4"><div><dt class="text-xs text-muted">{{ ['Booking', 'Practice', 'Qualifying', 'Race'][instance.session?.type ?? -1] || 'Session' }} · elapsed</dt><dd class="mt-1 font-mono text-2xl">{{ instance.session ? elapsed(instance) : '—' }}</dd></div><div><dt class="text-xs text-muted">Drivers connected</dt><dd class="mt-1 font-mono text-2xl">{{ instance.players }}</dd></div></dl>
          <div v-else class="mt-4 rounded-md border p-4 text-sm" :class="startable(instance) ? 'border-ok/30 bg-ok-glow' : 'border-warn/30 bg-warn-glow'">
            <p class="font-semibold" :class="startable(instance) ? 'text-ok' : 'text-warn'">{{ startable(instance) ? 'Ready to start' : !summary || readinessError ? 'Readiness unavailable' : !summary.can_start ? 'Setup needs attention' : 'Choose a race to run' }}</p>
            <p class="mt-1 text-muted">{{ !summary || readinessError ? 'Refresh status to check the installation and run plan.' : !summary.can_start ? summary.blocking[0]?.message : instance.run_mode === 'repeat_event' ? 'Repeats the pinned race. Your manual queue is preserved.' : pendingCount(instance.id) ? `${pendingCount(instance.id)} race${pendingCount(instance.id) === 1 ? '' : 's'} queued on this server.` : 'Nothing is queued on this server yet.' }}</p>
          </div>
          <p v-if="actionErrors[instance.id]" role="alert" class="mt-4 text-sm text-danger">{{ actionErrors[instance.id] }}</p>
        </div>
        <footer class="flex flex-wrap items-center justify-between gap-3 border-t border-line px-5 py-4">
          <RouterLink :to="queueTo(instance.id)" class="inline-flex min-h-11 items-center text-sm text-accent hover:underline">Run plan →</RouterLink>
          <RouterLink v-if="instance.running || !auth.canOperate" :to="controlTo(instance.id)" class="inline-flex min-h-11 items-center rounded-md bg-primary px-4 text-sm font-semibold text-white hover:brightness-110">Race Control →</RouterLink>
          <Button v-else-if="startable(instance)" :disabled="busy[instance.id] || !!detailErrors[instance.id]" @click="start(instance)"><Icon name="power" :size="16" />{{ busy[instance.id] ? 'Starting…' : 'Start ' + instance.name }}</Button>
          <RouterLink v-else-if="auth.isAdmin && summary && !summary.can_start" to="/setup" class="inline-flex min-h-11 items-center rounded-md border border-line bg-surface-2 px-4 text-sm font-medium">Finish server setup</RouterLink>
          <RouterLink v-else :to="queueTo(instance.id)" class="inline-flex min-h-11 items-center rounded-md border border-line bg-surface-2 px-4 text-sm font-medium">Choose race setup →</RouterLink>
        </footer>
      </article>
    </div>
  </template>
  <section v-if="!loading && channels.length" class="mt-6 rounded-md border border-line bg-surface p-5">
    <div class="flex flex-wrap items-center justify-between gap-3"><div><h2 class="text-base font-semibold">Driver streams <span class="ml-2 text-sm font-normal text-muted">{{ onlineChannels.length }} live</span></h2><p v-if="!onlineChannels.length" class="mt-1 text-sm text-muted">No streams online. Live video appears here when available.</p></div><div class="flex flex-wrap gap-2"><Button v-if="onlineChannels.length" variant="dark" @click="watchStream()">Theater</Button><button type="button" class="min-h-11 text-sm text-accent underline" :aria-expanded="showOfflineStreams" @click="showOfflineStreams = !showOfflineStreams">{{ showOfflineStreams ? 'Hide offline streams' : 'Show all streams' }}</button></div></div>
    <StreamWall v-if="onlineChannels.length || showOfflineStreams" class="mt-4" :channels="showOfflineStreams ? channels : onlineChannels" @watch="watchStream" />
  </section>
  <LiveFeed class="mt-6" />
  <StreamTheater :open="theaterOpen" :channels="channels" :initial-key="theaterKey" @close="theaterOpen = false" />
</template>
