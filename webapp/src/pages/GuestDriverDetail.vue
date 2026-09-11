<script setup lang="ts">
// Guest profiles share the Pitlane people layout while retaining their own
// attribution, history, photo and media operations.
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { describeResult, fmtDate, fmtScore, sessionKindLabel, timeAgo } from "@/lib/driversApi";
import { getGuestDriver } from "@/lib/guestDriversApi";
import { lapTime } from "@/lib/raceTelemetry";
import { ApiError, csrfToken } from "@/lib/api";
import { useToastStore } from "@/stores/toast";
import { useAuthStore } from "@/stores/auth";
import { useContentStore } from "@/stores/content";
import type { GuestDriverDetail, MediaItem } from "@/types/driverStats";
import Card from "@/components/ui/Card.vue";
import Icon from "@/components/ui/Icon.vue";
import DriverAvatar from "@/components/ui/DriverAvatar.vue";
import Sparkline from "@/components/ui/Sparkline.vue";
import DriverFavourites from "@/components/DriverFavourites.vue";
import ManualRecordings from "@/components/ManualRecordings.vue";

const route = useRoute();
const toast = useToastStore();
const auth = useAuthStore();
const content = useContentStore();

const id = computed(() => Number(route.params.id));
const guest = ref<GuestDriverDetail | null>(null);
const loading = ref(true);

// --- avatar upload (local preview until the backend persists it) -------------
const fileInput = ref<HTMLInputElement | null>(null);
const localAvatar = ref<string | null>(null);
const avatarSrc = computed(() => localAvatar.value ?? guest.value?.avatar_url ?? null);

function pickPhoto() {
  fileInput.value?.click();
}
async function onPickFile(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  if (localAvatar.value) URL.revokeObjectURL(localAvatar.value);
  localAvatar.value = URL.createObjectURL(file); // instant local preview
  input.value = ""; // let the same file be re-picked later
  try {
    const fd = new FormData();
    fd.append("file", file);
    // Raw fetch: the JSON api client can't send multipart. Mirror its CSRF header.
    const res = await fetch(`/api/guest-drivers/${id.value}/avatar`, {
      method: "POST",
      headers: { "X-CSRF-Token": csrfToken() },
      body: fd,
    });
    if (res.ok) toast.success("Guest photo updated.");
    else toast.error("Could not save the photo.");
  } catch {
    toast.error("Could not save the photo.");
  }
}

// --- derived data ------------------------------------------------------------
const lastActive = computed(() => {
  const g = guest.value;
  if (!g) return 0;
  return g.last_seen > g.created_at ? g.last_seen : 0;
});

function isRealMedia(m: MediaItem): boolean {
  return !!m.url && m.url !== "#";
}
function downloadMedia(m: MediaItem) {
  if (!isRealMedia(m)) return;
  const href = m.url + (m.url.includes("?") ? "&" : "?") + "download=1";
  const a = document.createElement("a");
  a.href = href;
  a.rel = "noopener";
  document.body.appendChild(a);
  a.click();
  a.remove();
}

async function load() {
  loading.value = true;
  try {
    guest.value = await getGuestDriver(id.value);
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
    guest.value = null;
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  void content.load(); // car list backs the favourite-car preview
  void load();
});
watch(id, load);
onBeforeUnmount(() => {
  if (localAvatar.value) URL.revokeObjectURL(localAvatar.value);
});
</script>

<template>
  <RouterLink
    :to="{ name: auth.canOperate ? 'guest-drivers' : 'drivers' }"
    class="pitlane-back"
  >
    <Icon name="arrowLeft" :size="16" />
    {{ auth.canOperate ? "Guest roster" : "Drivers" }}
  </RouterLink>

  <!-- Loading -->
  <div v-if="loading" class="space-y-4">
    <div class="h-44 animate-pulse rounded-lg border border-line bg-surface-2/50" />
    <div class="grid gap-4 md:grid-cols-2">
      <div class="h-40 animate-pulse rounded-md border border-line bg-surface-2/50" />
      <div class="h-40 animate-pulse rounded-md border border-line bg-surface-2/50" />
    </div>
  </div>

  <!-- Not found -->
  <Card v-else-if="!guest">
    <div class="py-12 text-center">
      <div class="mx-auto grid size-12 place-items-center rounded-lg border border-line bg-surface-2 text-dim">
        <Icon name="alert" :size="22" />
      </div>
      <p class="mt-3 text-sm font-semibold text-text">Guest driver not found</p>
      <p class="mt-0.5 text-sm text-muted">This guest may have been removed from the roster.</p>
    </div>
  </Card>

  <template v-else>
    <header class="pitlane-profile-heading">
      <div class="pitlane-profile-photo">
        <DriverAvatar :name="guest.name" :src="avatarSrc" :size="104" />
        <button v-if="auth.canOperate" type="button" class="pitlane-photo-edit" title="Upload photo" aria-label="Upload driver photo" @click="pickPhoto"><Icon name="upload" :size="18" /></button>
        <input ref="fileInput" type="file" accept="image/*" class="hidden" @change="onPickFile" />
      </div>
      <div class="min-w-0"><span class="pitlane-eyebrow">Your drivers · Guest profile</span>
          <h1>{{ guest.name }}</h1>
          <p>Shared simulator driver · Added {{ fmtDate(guest.created_at) }}</p>
          <p v-if="lastActive">Last drove {{ timeAgo(lastActive) }}</p>
          <p v-if="guest.notes" class="pitlane-profile-notes">{{ guest.notes }}</p></div>
    </header>
    <dl class="pitlane-metrics" aria-label="Driver statistics">
      <div><dt>Best drift</dt><dd>{{ guest.best_drift ? fmtScore(guest.best_drift) : '—' }}</dd></div>
      <div><dt>Fastest lap</dt><dd>{{ guest.best_lap_ms ? lapTime(guest.best_lap_ms) : '—' }}</dd></div>
      <div><dt>Laps completed</dt><dd>{{ guest.total_laps.toLocaleString() }}</dd></div>
      <div><dt>Sessions with the club</dt><dd>{{ guest.sessions.toLocaleString() }}</dd></div>
      <div><dt>Podiums</dt><dd>{{ guest.podiums.toLocaleString() }}</dd></div>
    </dl>
    <div v-if="guest.drift_trend.length" class="pitlane-profile-trend"><span>Recent drift runs</span><Sparkline :values="guest.drift_trend" :width="300" :height="34" class="text-accent max-w-full" /></div>

    <DriverFavourites :car="guest.favourite_car" :track="guest.favourite_track" />

    <div class="profile-story-grid">
      <section class="min-w-0" aria-labelledby="guest-drives-title">
        <div class="profile-section-heading"><div><h2 id="guest-drives-title">Recent drives</h2><p>Every result, credited to the person at the wheel.</p></div></div>
        <ul v-if="guest.results.length" class="profile-drive-list">
          <li v-for="r in guest.results" :key="r.session_id" class="profile-guest-drive">
            <div><span class="pitlane-eyebrow">{{ sessionKindLabel[r.kind] }} · {{ fmtDate(r.date) }}</span><h3>{{ r.track.name }}</h3><p>{{ r.car.name }}</p></div>
            <div class="profile-drive-metric"><b :class="describeResult(r).tone === 'warn' ? 'text-warn' : describeResult(r).tone === 'accent' ? 'text-accent' : ''">{{ describeResult(r).primary }}</b><span>{{ describeResult(r).unit }}</span></div>
          </li>
        </ul>
        <div v-else class="profile-quiet-empty"><h3>The first drive is still to come.</h3><p>Results recorded under this guest's name will appear here.</p></div>
      </section>
      <aside class="profile-aside">
        <ManualRecordings v-if="guest.media.length" :items="guest.media" heading="Personal highlights" context="Clips and pictures from their drives." :account-label="guest.name" :guests="[{ ...guest, id }]" :can-operate="false" @download-media="downloadMedia" />
        <section v-else>
          <div class="profile-section-heading"><h2>Personal highlights</h2></div>
          <div class="profile-quiet-empty"><h3>The next good moment is out there.</h3><p>Clips and pictures credited to this driver will appear here.</p></div>
        </section>
      </aside>
    </div>
  </template>
</template>
