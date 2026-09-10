<script setup lang="ts">
import { toRef, useId } from "vue";
import { useDialog } from "@/lib/useDialog";
import Icon from "@/components/ui/Icon.vue";

const props = defineProps<{ open: boolean; title?: string; wide?: boolean }>();
const emit = defineEmits<{ close: [] }>();
const dialog = useDialog(toRef(props, "open"));
const titleId = useId();
</script>

<template>
  <Teleport to="body">
    <dialog ref="dialog" :aria-labelledby="titleId" class="app-dialog" @cancel.prevent="emit('close')" @click.self="emit('close')">
      <div v-if="open" class="flex max-h-[90dvh] w-full flex-col overflow-hidden rounded-lg border border-line bg-surface text-text shadow-2xl" :class="wide ? 'max-w-5xl' : 'max-w-lg'">
        <header class="flex items-center gap-3 border-b border-line bg-rail px-6 py-4">
          <h2 :id="titleId" class="text-lg font-semibold">{{ title || 'Dialog' }}</h2>
          <button type="button" class="ml-auto grid size-11 shrink-0 cursor-pointer place-items-center rounded-md text-muted hover:bg-surface-2 hover:text-text" aria-label="Close dialog" @click="emit('close')"><Icon name="x" :size="18" /></button>
        </header>
        <div class="min-h-0 flex-1 overflow-y-auto p-5"><slot /></div>
        <footer v-if="$slots.footer" class="flex flex-wrap items-center justify-end gap-2 border-t border-line bg-rail px-6 py-4"><slot name="footer" /></footer>
      </div>
    </dialog>
  </Teleport>
</template>
