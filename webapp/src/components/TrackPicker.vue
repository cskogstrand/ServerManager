<script setup lang="ts">
// Searchable track grid in a side sheet. Emits the selected track key+config.
import { computed, onMounted, ref } from "vue";
import { useContentStore } from "@/stores/content";
import Sheet from "@/components/ui/Sheet.vue";
import Input from "@/components/ui/Input.vue";
import TrackImage from "@/components/TrackImage.vue";

const props = defineProps<{
  open: boolean;
  selectedKey?: string | null;
  selectedConfig?: string | null;
}>();

const emit = defineEmits<{
  close: [];
  select: [track: { key: string; config: string; name: string; pitboxes: number }];
}>();

const content = useContentStore();
onMounted(() => void content.load());

const search = ref("");

const filtered = computed(() =>
  content.tracks.filter((t) =>
    `${t.name ?? ""} ${t.key ?? ""} ${t.config ?? ""} ${t.version ?? ""}`.toLowerCase().includes(search.value.toLowerCase()),
  ),
);

function isSelected(t: { key?: string; config?: string }) {
  return t.key === props.selectedKey && (t.config ?? "") === (props.selectedConfig ?? "");
}
</script>

<template>
  <Sheet :open="open" title="Choose track" @close="emit('close')">
    <Input aria-label="Search tracks" v-model="search" placeholder="Search tracks…" class="mb-3" />

    <div class="grid grid-cols-2 gap-3">
      <button
        v-for="t in filtered"
        :key="`${t.key}:${t.config}`"
        type="button"
        class="cursor-pointer overflow-hidden rounded-md border bg-surface-2 text-left transition-colors duration-200 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
        :class="isSelected(t) ? 'border-accent ring-1 ring-accent/25' : 'border-line hover:border-line-hi'"
        @click="
          emit('select', {
            key: t.key ?? '',
            config: t.config ?? '',
            name: t.name ?? t.key ?? '',
            pitboxes: t.pitboxes ?? 0,
          })
        "
      >
        <TrackImage
          :track-key="t.key"
          :config="t.config"
          class="aspect-video w-full"
        />
        <div class="p-2">
          <div class="truncate text-sm font-medium">{{ t.name }}</div>
          <div class="text-xs text-dim">
            {{ t.config || "default" }}<span v-if="t.version"> · version {{ t.version }}</span> · {{ t.pitboxes }} pits
          </div>
        </div>
      </button>
    </div>
  </Sheet>
</template>
