<script setup lang="ts">
// Always-visible download / delete controls for a highlight tile (clip or
// screenshot). Rendered inline in the tile footer next to the caption; the
// parent owns the actual download/delete logic and just listens for the events.
import type { MediaItem } from "@/types/driverStats";
import Icon from "@/components/ui/Icon.vue";

defineProps<{
  item: MediaItem;
  canDelete: boolean;
  deleting: boolean;
}>();

defineEmits<{
  (e: "download"): void;
  (e: "delete"): void;
}>();
</script>

<template>
  <div class="flex shrink-0 gap-1">
    <button
      type="button"
      title="Download"
      :aria-label="`Download ${item.kind === 'clip' ? 'clip' : 'screenshot'}`"
      class="grid size-11 place-items-center rounded-md border border-line bg-surface-2 text-muted transition-colors hover:border-accent/50 hover:text-accent"
      @click.stop="$emit('download')"
    >
      <Icon name="download" :size="14" />
    </button>
    <button
      v-if="canDelete"
      type="button"
      title="Delete"
      :aria-label="`Delete ${item.kind === 'clip' ? 'clip' : 'screenshot'}`"
      :disabled="deleting"
      class="grid size-11 place-items-center rounded-md border border-line bg-surface-2 text-muted transition-colors hover:border-danger/60 hover:text-danger disabled:opacity-50"
      @click.stop="$emit('delete')"
    >
      <Icon name="trash" :size="14" />
    </button>
  </div>
</template>
