<script setup lang="ts">
// Manual recordings reel — clips/snapshots a driver captured outside any session
// (no connection). Each tile can be played, downloaded, deleted, and attributed
// to a guest driver ("Driven by"), since a manual clip often credits a passenger
// or a different real person sharing the account.
import { ref, watch } from "vue";
import { fmtScore, timeAgo } from "@/lib/driversApi";
import { fmtClipDuration } from "@/lib/useDriverCapture";
import type { MediaItem } from "@/types/driverStats";
import type { GuestDriver } from "@/lib/guestDriversApi";
import Icon from "@/components/ui/Icon.vue";
import MediaActions from "@/components/MediaActions.vue";
import Modal from "@/components/ui/Modal.vue";
import Button from "@/components/ui/Button.vue";

const props = defineProps<{
  items: MediaItem[];
  heading?: string;
  context?: string;
  initialFile?: string;
  accountLabel?: string;
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
let openedFile = "";
watch(() => [props.initialFile, props.items] as const, ([file, items]) => {
  if (file && file !== openedFile) {
    const item = items.find(m => m.url.split("/").pop() === file);
    if (item) { active.value = item; openedFile = file; }
  }
}, { immediate: true });
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
  <section v-if="items.length" class="profile-recordings">
    <div class="profile-section-heading">
      <div><h2>{{ heading || 'Saved moments' }}</h2><p>{{ context ?? 'Clips and pictures captured outside a drive.' }}</p></div>
      <span class="text-sm text-muted">{{ items.length }}</span>
    </div>
    <div class="profile-recording-grid">
      <article v-for="m in items" :key="m.id">
        <button type="button" class="profile-capture-preview" :disabled="!isRealMedia(m)" :aria-label="`${m.kind === 'clip' ? 'Play' : 'View'} ${m.caption || (m.kind === 'clip' ? 'clip' : 'screenshot')}`" @click="openMedia(m)">
          <img v-if="m.kind === 'screenshot' && isRealMedia(m)" :src="m.url" alt="" loading="lazy" />
          <img v-else-if="m.thumb_url" :src="m.thumb_url" alt="" loading="lazy" />
          <video v-else-if="isRealMedia(m)" :src="m.url" preload="metadata" muted playsinline />
          <Icon v-else :name="m.kind === 'clip' ? 'play' : 'camera'" :size="28" />
          <span v-if="m.kind === 'clip'" class="profile-capture-play"><Icon name="play" :size="22" />{{ m.duration_s ? fmtClipDuration(m.duration_s) : 'Play clip' }}</span>
        </button>
        <div class="profile-capture-caption">
          <div><b>{{ m.caption || (m.kind === 'clip' ? 'Clip' : 'Screenshot') }}</b><p>{{ timeAgo(m.captured_at) }}<span v-if="m.trigger"> · +{{ fmtScore(m.trigger.delta) }} points</span></p></div>
          <MediaActions v-if="isRealMedia(m)" :item="m" :can-delete="canOperate" :deleting="deletingIds?.has(m.id) ?? false" @download="emit('download-media', m)" @delete="emit('delete-media', m)" />
        </div>
        <button v-if="canOperate" type="button" class="profile-recording-driver" @click="assignFor = m"><span>Driven by <b>{{ guestName(m) || accountLabel || 'Account driver' }}</b></span><Icon name="arrowDown" :size="16" /></button>
        <p v-else class="profile-recording-credit">Driven by {{ guestName(m) || accountLabel || 'Account driver' }}</p>
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
            <Icon name="user" :size="14" /> {{ accountLabel || "Account driver (own name)" }}
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
