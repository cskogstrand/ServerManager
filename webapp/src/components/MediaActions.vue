<script setup lang="ts">
// Hover-revealed download / delete controls for a highlight tile (clip or
// screenshot). Sits over the bottom-right of the media area; the parent owns
// the actual download/delete logic and just listens for the events.
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
  <div
    class="absolute right-2 bottom-2 flex gap-1 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100"
  >
    <button
      type="button"
      title="Download"
      class="grid size-7 place-items-center rounded-md border border-line bg-bg/70 text-text/90 backdrop-blur-sm transition-colors hover:border-accent/50 hover:text-accent"
      @click.stop="$emit('download')"
    >
      <Icon name="download" :size="14" />
    </button>
    <button
      v-if="canDelete"
      type="button"
      title="Delete"
      :disabled="deleting"
      class="grid size-7 place-items-center rounded-md border border-line bg-bg/70 text-text/90 backdrop-blur-sm transition-colors hover:border-danger/60 hover:text-danger disabled:opacity-50"
      @click.stop="$emit('delete')"
    >
      <Icon name="trash" :size="14" />
    </button>
  </div>
</template>
