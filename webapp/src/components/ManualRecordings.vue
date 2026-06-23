<script setup lang="ts">
// Manual recordings reel — clips/snapshots a driver captured outside any session
// (no connection). Each tile can be played, downloaded, deleted, and attributed
// to a guest driver ("Driven by"), since a manual clip often credits a passenger
// or a different real person sharing the account.
import { ref } from "vue";
import { timeAgo } from "@/lib/driversApi";
import { fmtClipDuration } from "@/lib/useDriverCapture";
import type { MediaItem } from "@/types/driverStats";
import type { GuestDriver } from "@/lib/guestDriversApi";
import Icon from "@/components/ui/Icon.vue";
import MediaActions from "@/components/MediaActions.vue";
import Modal from "@/components/ui/Modal.vue";
import Button from "@/components/ui/Button.vue";

const props = defineProps<{
  items: MediaItem[];
  canOperate: boolean;
  deletingIds?: Set<string>;
  guests?: GuestDriver[];
}>();

const emit = defineEmits<{
  (e: "delete-media", media: MediaItem): void;
  (e: "download-media", media: MediaItem): void;
  (e: "assign", media: MediaItem, guestDriverId: number | null): void;
}>();

function isRealMedia(m: MediaItem): boolean {
  return !!m.url && m.url !== "#";
}

function guestName(m: MediaItem): string {
  if (!m.guest_driver_id) return "";
  return props.guests?.find((g) => g.id === m.guest_driver_id)?.name ?? "Guest driver";
}

// One lightbox for the whole reel.
const active = ref<MediaItem | null>(null);
function openMedia(m: MediaItem) {
  if (isRealMedia(m)) active.value = m;
}

// Assign modal, scoped to the tile that opened it.
const assignFor = ref<MediaItem | null>(null);
function pick(guestDriverId: number | null) {
  const m = assignFor.value;
  assignFor.value = null;
  if (m) emit("assign", m, guestDriverId);
}
</script>

<template>
  <section v-if="items.length" class="reveal">
    <div class="mb-2 flex items-center gap-2">
      <h2 class="flex items-center gap-2 text-sm font-bold tracking-tight">
        <Icon name="film" :size="15" class="text-accent" /> Manual recordings
        <span class="text-xs font-normal text-dim">not tied to a session</span>
      </h2>
      <span class="font-mono text-xs text-dim">({{ items.length }})</span>
    </div>

    <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
      <article
        v-for="m in items"
        :key="m.id"
        class="group overflow-hidden rounded-lg border border-line bg-surface-2/30"
      >
        <button
          type="button"
          class="relative block aspect-video w-full overflow-hidden bg-black"
          :class="isRealMedia(m) ? 'cursor-pointer' : 'cursor-default'"
          @click="openMedia(m)"
        >
          <img v-if="m.kind === 'screenshot' && isRealMedia(m)" :src="m.url" alt="" loading="lazy" class="size-full object-cover" />
          <img v-else-if="m.thumb_url" :src="m.thumb_url" alt="" loading="lazy" class="size-full object-cover" />
          <video v-else-if="isRealMedia(m)" :src="m.url" class="size-full object-cover" preload="metadata" muted playsinline />
          <div v-else class="grid size-full place-items-center text-text/25">
            <Icon :name="m.kind === 'clip' ? 'play' : 'camera'" :size="20" />
          </div>
          <span
            v-if="m.kind === 'clip' && isRealMedia(m)"
            class="absolute inset-0 grid place-items-center bg-black/25 text-white/90 transition-colors group-hover:bg-black/40"
          >
            <Icon name="play" :size="20" />
          </span>
          <span
            v-if="m.duration_s"
            class="pointer-events-none absolute right-1 bottom-1 rounded bg-black/60 px-1 font-mono text-[9px] font-semibold text-white/90"
          >{{ fmtClipDuration(m.duration_s) }}</span>
        </button>

        <div class="flex flex-col gap-1.5 p-2">
          <div class="flex items-center gap-1.5">
            <div class="min-w-0 flex-1 truncate text-[10px] text-muted">{{ timeAgo(m.captured_at) }}</div>
            <MediaActions
              v-if="isRealMedia(m)"
              :item="m"
              :can-delete="canOperate"
              :deleting="deletingIds?.has(m.id) ?? false"
              @download="emit('download-media', m)"
              @delete="emit('delete-media', m)"
            />
          </div>
          <!-- Driven by: per-recording guest attribution -->
          <button
            type="button"
            class="flex items-center gap-1.5 rounded-md border border-line/60 bg-surface/40 px-2 py-1 text-left text-[11px] transition-colors"
            :class="canOperate ? 'cursor-pointer hover:border-accent/50' : 'cursor-default'"
            :disabled="!canOperate"
            @click="canOperate && (assignFor = m)"
          >
            <Icon name="user" :size="12" class="shrink-0 text-dim" />
            <span class="text-muted">Driven by</span>
            <span class="min-w-0 flex-1 truncate font-semibold" :class="guestName(m) ? 'text-accent' : 'text-text'">
              {{ guestName(m) || "Account driver" }}
            </span>
            <Icon v-if="canOperate" name="arrowDown" :size="12" class="shrink-0 text-dim" />
          </button>
        </div>
      </article>
    </div>

    <!-- Lightbox -->
    <Modal :open="!!active" wide :title="active?.caption || (active?.kind === 'clip' ? 'Clip' : 'Capture')" @close="active = null">
      <video
        v-if="active?.kind === 'clip'"
        :src="active.url"
        class="mx-auto max-h-[78vh] w-full rounded-md bg-black object-contain"
        controls
        autoplay
        playsinline
      />
      <img v-else-if="active" :src="active.url" :alt="active.caption" class="mx-auto max-h-[78vh] w-full rounded-md object-contain" />
      <template #footer>
        <Button v-if="active && isRealMedia(active)" variant="ghost" @click="emit('download-media', active)">
          <Icon name="download" :size="14" class="mr-1.5" /> Download
        </Button>
        <Button @click="active = null">Close</Button>
      </template>
    </Modal>

    <!-- Assign-driver modal, scoped to one recording -->
    <Modal :open="!!assignFor" title="Assign recording to driver" @close="assignFor = null">
      <p class="mb-3 text-xs text-muted">
        Attribute this recording to a driver. Pick the account's own name to revert.
      </p>
      <ul class="space-y-1">
        <li>
          <button
            type="button"
            class="flex w-full items-center gap-2 rounded-md border px-3 py-2 text-left text-sm transition-colors"
            :class="!assignFor?.guest_driver_id ? 'border-accent/50 bg-accent-dim font-semibold text-accent' : 'border-line text-text hover:border-line-hi hover:bg-surface-2'"
            @click="pick(null)"
          >
            <Icon name="user" :size="14" /> Account driver (own name)
          </button>
        </li>
        <li v-for="g in guests" :key="g.id">
          <button
            type="button"
            class="flex w-full items-center gap-2 rounded-md border px-3 py-2 text-left text-sm transition-colors"
            :class="assignFor?.guest_driver_id === g.id ? 'border-accent/50 bg-accent-dim font-semibold text-accent' : 'border-line text-text hover:border-line-hi hover:bg-surface-2'"
            @click="pick(g.id)"
          >
            {{ g.name }}
          </button>
        </li>
        <li v-if="!guests || !guests.length" class="px-3 py-2 text-xs text-dim">
          No guest drivers yet — add them on the Guest Drivers page.
        </li>
      </ul>
    </Modal>
  </section>
</template>

<style scoped>
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
