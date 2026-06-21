<script setup lang="ts">
// Guest driver detail — the profile + highlight reel for one roster guest (a
// real person who shares an account GUID). Mirrors the GUID DriverDetail hero,
// KPIs, favourites and session history, but a guest has no GUID, no live
// telemetry and no stream of their own — so there is no "current race", stream
// player or capture controls. A large badge marks the page as a guest profile.
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { describeResult, fmtDate, fmtScore, sessionKindLabel, timeAgo } from "@/lib/driversApi";
import { getGuestDriver } from "@/lib/guestDriversApi";
import { lapTime } from "@/lib/raceTelemetry";
import { ApiError, csrfToken } from "@/lib/api";
import { useToastStore } from "@/stores/toast";
import { useAuthStore } from "@/stores/auth";
import { useContentStore } from "@/stores/content";
import type { GuestDriverDetail, DriverResult, MediaItem } from "@/types/driverStats";
import Card from "@/components/ui/Card.vue";
import Icon from "@/components/ui/Icon.vue";
import DriverAvatar from "@/components/ui/DriverAvatar.vue";
import Sparkline from "@/components/ui/Sparkline.vue";
import CountUp from "@/components/ui/CountUp.vue";
import TrackImage from "@/components/TrackImage.vue";
import MediaActions from "@/components/MediaActions.vue";

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
const clips = computed(() => guest.value?.media.filter((m) => m.kind === "clip") ?? []);
const screenshots = computed(() => guest.value?.media.filter((m) => m.kind === "screenshot") ?? []);
const lastActive = computed(() => {
  const g = guest.value;
  if (!g) return 0;
  return g.last_seen > g.created_at ? g.last_seen : 0;
});

// Resolve the favourite car's preview against the cached car list (same source
// the Content grid renders from) so the image matches even when the recorded
// skin is blank. Falls back to the recorded skin if the car isn't cached.
const carImgUrl = computed(() => {
  const c = guest.value?.favourite_car;
  if (!c) return "";
  const car = content.carByKey(c.key);
  const skin = car?.skins.find((s) => s.key === c.skin)?.key ?? car?.skins[0]?.key ?? c.skin ?? "";
  return `/api/car/image/${encodeURIComponent(c.key)}/${encodeURIComponent(skin)}?v=${content.imageVersion}`;
});
const carImgOk = ref(true);
watch(carImgUrl, () => {
  carImgOk.value = true;
});

const tileTints = [
  "linear-gradient(135deg, rgba(98,179,232,0.18), rgba(16,26,37,0.94))",
  "linear-gradient(135deg, rgba(150,140,232,0.16), rgba(16,26,37,0.94))",
  "linear-gradient(135deg, rgba(96,202,202,0.16), rgba(16,26,37,0.94))",
];
const tileStyle = (i: number) => ({ background: tileTints[i % tileTints.length] });

function kindBadge(r: DriverResult): string {
  if (r.kind === "drift") return "border-accent/40 bg-accent-dim text-accent";
  if (r.kind === "race" && r.position === 1) return "border-warn/45 bg-warn-glow text-warn";
  if (r.kind === "race") return "border-line-hi bg-surface-3 text-text";
  return "border-line bg-surface-2 text-muted";
}

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
  carImgOk.value = true;
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
    :to="{ name: 'guest-drivers' }"
    class="mb-4 inline-flex items-center gap-1.5 text-sm font-semibold text-muted transition-colors hover:text-accent"
  >
    <Icon name="arrowLeft" :size="16" />
    Guest Drivers
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
    <!-- HERO -->
    <section class="hero reveal relative overflow-hidden rounded-lg border border-line">
      <div class="hero-grid" />
      <div class="relative flex flex-col gap-5 p-5 md:flex-row md:items-start md:p-6">
        <!-- avatar + identity -->
        <div class="flex items-center gap-4">
          <div class="relative">
            <DriverAvatar :name="guest.name" :src="avatarSrc" :size="137" radius="lg" />
            <button
              v-if="auth.canOperate"
              type="button"
              class="absolute -right-1.5 -bottom-1.5 grid size-7 cursor-pointer place-items-center rounded-md border border-line-hi bg-surface-3 text-muted transition-colors hover:border-accent/60 hover:text-accent"
              title="Upload photo"
              @click="pickPhoto"
            >
              <Icon name="upload" :size="14" />
            </button>
            <input ref="fileInput" type="file" accept="image/*" class="hidden" @change="onPickFile" />
          </div>
          <div class="min-w-0">
            <!-- the large "this is a guest" badge -->
            <span
              class="mb-2 inline-flex items-center gap-1.5 rounded-md border border-warn/45 bg-warn-glow px-2.5 py-1 text-xs font-black tracking-[0.14em] text-warn uppercase"
            >
              <Icon name="users" :size="14" /> Guest driver
            </span>
            <h1 class="text-2xl font-black tracking-tight text-text">{{ guest.name }}</h1>
            <div v-if="guest.notes" class="mt-1 truncate text-sm text-muted">{{ guest.notes }}</div>
            <div class="mt-1 text-xs text-muted">
              Added {{ fmtDate(guest.created_at) }}<span v-if="lastActive"> · last raced {{ timeAgo(lastActive) }}</span>
            </div>
          </div>
        </div>

        <!-- KPIs + trend -->
        <div class="md:ml-auto md:max-w-125 md:flex-1">
          <div class="grid grid-cols-3 gap-2 sm:grid-cols-5 md:grid-cols-3 lg:grid-cols-5">
            <div class="rounded-md border border-line bg-surface/70 px-3 py-2">
              <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Best drift</div>
              <div class="font-mono text-lg font-bold tabular-nums text-accent"><CountUp :value="guest.best_drift" /></div>
            </div>
            <div class="col-span-2 rounded-md border border-line bg-surface/70 px-3 py-2">
              <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Fastest lap</div>
              <div class="truncate font-mono text-lg font-bold tabular-nums text-text">{{ guest.best_lap_ms ? lapTime(guest.best_lap_ms) : "—" }}</div>
            </div>
            <div class="rounded-md border border-line bg-surface/70 px-3 py-2">
              <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Laps</div>
              <div class="font-mono text-lg font-bold tabular-nums text-text"><CountUp :value="guest.total_laps" /></div>
            </div>
            <div class="rounded-md border border-line bg-surface/70 px-3 py-2">
              <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Sessions</div>
              <div class="font-mono text-lg font-bold tabular-nums text-text"><CountUp :value="guest.sessions" /></div>
            </div>
            <div class="rounded-md border border-line bg-surface/70 px-3 py-2">
              <div class="text-[10px] font-bold tracking-wide text-dim uppercase">Podiums</div>
              <div class="font-mono text-lg font-bold tabular-nums" :class="guest.podiums ? 'text-warn' : 'text-text'"><CountUp :value="guest.podiums" /></div>
            </div>
          </div>
          <div class="mt-2 flex items-center gap-3 rounded-md border border-line bg-surface/70 px-3 py-2 text-accent">
            <span class="text-[10px] font-bold tracking-wide text-dim uppercase">Drift trend</span>
            <Sparkline :values="guest.drift_trend" :width="300" :height="34" class="ml-auto h-auto min-w-0 max-w-full" />
          </div>
        </div>
      </div>
    </section>

    <!-- FAVOURITES -->
    <div class="mt-4 grid gap-4 md:grid-cols-2">
      <Card class="min-w-0">
        <template #header>
          <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
            <Icon name="star" :size="15" class="text-warn" /> Favourite car
          </h2>
        </template>
        <div v-if="guest.favourite_car" class="flex items-center gap-3 sm:gap-4">
          <div class="grid h-16 w-24 shrink-0 place-items-center overflow-hidden rounded-md border border-line bg-surface-2 sm:h-20 sm:w-32">
            <img v-if="carImgOk" :src="carImgUrl" alt="" class="size-full object-cover" @error="carImgOk = false" />
            <Icon v-else name="car" :size="28" class="text-dim" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="truncate text-base font-bold text-text">{{ guest.favourite_car.name }}</div>
            <div v-if="guest.favourite_car.skin" class="truncate text-xs text-muted">Livery · {{ guest.favourite_car.skin }}</div>
            <div class="mt-1 text-[11px] font-semibold tracking-wide text-dim uppercase">Most-driven car</div>
          </div>
        </div>
        <p v-else class="text-sm text-muted">No car data yet.</p>
      </Card>

      <Card class="min-w-0">
        <template #header>
          <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
            <Icon name="mapPin" :size="15" class="text-accent" /> Favourite track
          </h2>
        </template>
        <div v-if="guest.favourite_track" class="flex items-center gap-3 sm:gap-4">
          <TrackImage
            :track-key="guest.favourite_track.key"
            :config="guest.favourite_track.config"
            class="h-16 w-24 shrink-0 rounded-md border border-line bg-surface-2 sm:h-20 sm:w-32"
          />
          <div class="min-w-0 flex-1">
            <div class="truncate text-base font-bold text-text">{{ guest.favourite_track.name }}</div>
            <div v-if="guest.favourite_track.country" class="truncate text-xs text-muted">{{ guest.favourite_track.country }}</div>
            <div class="mt-1 text-[11px] font-semibold tracking-wide text-dim uppercase">Most-raced layout</div>
          </div>
        </div>
        <p v-else class="text-sm text-muted">No track data yet.</p>
      </Card>
    </div>

    <!-- HISTORY -->
    <Card class="mt-4 min-w-0">
      <template #header>
        <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
          <Icon name="activity" :size="15" /> Recent sessions
        </h2>
        <span class="text-xs text-dim">drift & timed</span>
      </template>
      <ul v-if="guest.results.length" class="-my-1">
        <li
          v-for="(r, i) in guest.results"
          :key="r.session_id"
          class="reveal flex min-w-0 items-center gap-3 border-b border-line/60 py-2.5 last:border-0"
          :style="{ animationDelay: Math.min(i, 12) * 35 + 'ms' }"
        >
          <span
            class="grid w-16 shrink-0 place-items-center rounded-md border py-1 text-[10px] font-bold tracking-wide uppercase"
            :class="kindBadge(r)"
          >
            {{ sessionKindLabel[r.kind] }}
          </span>
          <div class="min-w-0 flex-1">
            <div class="truncate text-sm font-semibold text-text">{{ r.track.name }}</div>
            <div class="truncate text-xs text-dim">
              <Icon name="car" :size="12" class="-mt-px mr-1 inline" />{{ r.car.name }} · {{ fmtDate(r.date) }}
            </div>
          </div>
          <div class="shrink-0 text-right">
            <div
              class="font-mono text-sm font-bold tabular-nums"
              :class="describeResult(r).tone === 'warn' ? 'text-warn' : describeResult(r).tone === 'accent' ? 'text-accent' : 'text-text'"
            >
              {{ describeResult(r).primary }}
            </div>
            <div class="text-[10px] font-bold tracking-wide text-dim uppercase">{{ describeResult(r).unit }}</div>
          </div>
        </li>
      </ul>
      <p v-else class="py-6 text-center text-sm text-muted">No sessions recorded under this guest yet.</p>
    </Card>

    <!-- HIGHLIGHT REEL -->
    <Card class="mt-4">
      <template #header>
        <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
          <Icon name="film" :size="15" class="text-accent" /> Highlights
        </h2>
        <span class="hidden text-xs text-dim sm:inline">clips from their drift runs</span>
      </template>

      <div v-if="guest.media.length" class="space-y-5">
        <div v-if="clips.length">
          <div class="mb-2 flex items-center gap-2 text-[11px] font-bold tracking-wide text-dim uppercase">
            <Icon name="film" :size="13" /> Clips <span class="text-dim/70">({{ clips.length }})</span>
          </div>
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <article
              v-for="(m, i) in clips"
              :key="m.id"
              class="reveal group overflow-hidden rounded-md border border-line bg-surface-2/40"
              :style="{ animationDelay: i * 40 + 'ms' }"
            >
              <div class="relative aspect-video overflow-hidden" :style="!isRealMedia(m) ? tileStyle(i) : undefined">
                <video v-if="isRealMedia(m)" :src="m.url" class="size-full bg-black object-cover" preload="none" controls playsinline />
                <template v-else>
                  <div class="scanlines" />
                  <div class="absolute inset-0 grid place-items-center">
                    <span class="grid size-11 place-items-center rounded-full border border-text/20 bg-bg/40 text-text/85 backdrop-blur-sm">
                      <Icon name="play" :size="20" />
                    </span>
                  </div>
                </template>
                <span
                  v-if="m.trigger"
                  class="pointer-events-none absolute top-2 left-2 inline-flex items-center gap-1 rounded-md border border-warn/45 bg-warn-glow px-1.5 py-0.5 font-mono text-[10px] font-bold text-warn"
                >
                  <Icon name="arrowUp" :size="11" /> +{{ fmtScore(m.trigger.delta) }}
                </span>
              </div>
              <div class="flex items-center gap-2 px-2.5 py-1.5">
                <div class="min-w-0 flex-1">
                  <div class="truncate text-xs font-semibold text-text">{{ m.caption }}</div>
                  <div class="text-[10px] text-muted">{{ timeAgo(m.captured_at) }}</div>
                </div>
                <MediaActions v-if="isRealMedia(m)" :item="m" :can-delete="false" :deleting="false" @download="downloadMedia(m)" />
              </div>
            </article>
          </div>
        </div>

        <div v-if="screenshots.length">
          <div class="mb-2 flex items-center gap-2 text-[11px] font-bold tracking-wide text-dim uppercase">
            <Icon name="camera" :size="13" /> Screenshots <span class="text-dim/70">({{ screenshots.length }})</span>
          </div>
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <article
              v-for="(m, i) in screenshots"
              :key="m.id"
              class="reveal group overflow-hidden rounded-md border border-line bg-surface-2/40"
              :style="{ animationDelay: i * 40 + 'ms' }"
            >
              <div class="relative aspect-video overflow-hidden" :style="!isRealMedia(m) ? tileStyle(i + 1) : undefined">
                <img v-if="isRealMedia(m)" :src="m.url" alt="" loading="lazy" class="size-full bg-black object-cover" />
                <span
                  v-if="m.trigger"
                  class="pointer-events-none absolute top-2 left-2 inline-flex items-center gap-1 rounded-md border border-warn/45 bg-warn-glow px-1.5 py-0.5 font-mono text-[10px] font-bold text-warn"
                >
                  <Icon name="arrowUp" :size="11" /> +{{ fmtScore(m.trigger.delta) }}
                </span>
              </div>
              <div class="flex items-center gap-2 px-2.5 py-1.5">
                <div class="min-w-0 flex-1">
                  <div class="truncate text-[11px] font-semibold text-text">{{ m.caption }}</div>
                  <div class="text-[10px] text-muted">{{ timeAgo(m.captured_at) }}</div>
                </div>
                <MediaActions v-if="isRealMedia(m)" :item="m" :can-delete="false" :deleting="false" @download="downloadMedia(m)" />
              </div>
            </article>
          </div>
        </div>
      </div>

      <div v-else class="py-10 text-center">
        <div class="mx-auto grid size-12 place-items-center rounded-lg border border-line bg-surface-2 text-dim">
          <Icon name="film" :size="22" />
        </div>
        <p class="mt-3 text-sm font-semibold text-text">No highlights yet</p>
        <p class="mx-auto mt-0.5 max-w-md text-sm text-muted">
          Clips auto-captured on this guest's big drift runs show up here. Nothing has tripped the trigger yet.
        </p>
      </div>
    </Card>
  </template>
</template>

<style scoped>
.hero {
  background:
    radial-gradient(130% 150% at 0% 0%, rgba(98, 179, 232, 0.12), transparent 55%),
    linear-gradient(180deg, var(--color-surface-2), var(--color-surface));
}
.hero-grid {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image:
    linear-gradient(rgba(53, 80, 106, 0.18) 1px, transparent 1px),
    linear-gradient(90deg, rgba(53, 80, 106, 0.18) 1px, transparent 1px);
  background-size: 28px 28px;
  mask-image: radial-gradient(120% 120% at 100% 0%, #000 0%, transparent 70%);
}
.scanlines {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: repeating-linear-gradient(0deg, rgba(255, 255, 255, 0.035) 0 1px, transparent 1px 3px);
}
@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
.reveal {
  animation: rise 0.5s cubic-bezier(0.22, 0.61, 0.36, 1) both;
}
</style>
