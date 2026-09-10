<script setup lang="ts">
import { computed, inject, nextTick, ref, useId, watch } from "vue";
import { formFieldKey } from "@/lib/formField";
import Icon from "@/components/ui/Icon.vue";

const props = defineProps<{
  id?: string;
  options: { value: string | number; label: string }[];
  placeholder?: string;
  ariaLabel?: string;
}>();
const model = defineModel<string | number | null>();
const field = inject(formFieldKey, null);
const generatedId = useId();
const listId = `${generatedId}-options`;
const list = ref<HTMLElement | null>(null);
const open = ref(false);
const query = ref("");
const active = ref(0);
const selectedLabel = computed(() => props.options.find(o => o.value === model.value)?.label ?? "");
const filtered = computed(() => props.options.filter(o => o.label.toLowerCase().includes(query.value.trim().toLowerCase())));
const optionId = (index: number) => `${listId}-${index}`;

function openList() {
  query.value = "";
  open.value = true;
  active.value = Math.max(0, props.options.findIndex(o => o.value === model.value));
}
function choose(value: string | number) { model.value = value; open.value = false; query.value = ""; }
function closeList() { open.value = false; query.value = ""; }
function onKeydown(event: KeyboardEvent) {
  if (!open.value && ["ArrowDown", "ArrowUp", "Enter"].includes(event.key)) {
    event.preventDefault(); openList(); return;
  }
  if (!open.value) return;
  if (["ArrowDown", "ArrowUp", "Enter", "Escape"].includes(event.key)) event.preventDefault();
  if (event.key === "ArrowDown") active.value = Math.min(active.value + 1, filtered.value.length - 1);
  if (event.key === "ArrowUp") active.value = Math.max(active.value - 1, 0);
  if (event.key === "Enter" && filtered.value[active.value]) choose(filtered.value[active.value].value);
  if (event.key === "Escape") { event.stopPropagation(); closeList(); }
}
watch([active, open, filtered], async () => {
  if (active.value >= filtered.value.length) active.value = Math.max(0, filtered.value.length - 1);
  await nextTick();
  list.value?.querySelector(`[id="${optionId(active.value)}"]`)?.scrollIntoView?.({ block: "nearest" });
});
</script>

<template>
  <div class="relative min-w-0">
    <input
      :id="id ?? field?.id.value ?? generatedId"
      role="combobox" aria-autocomplete="list" aria-haspopup="listbox"
      :aria-expanded="open" :aria-controls="open ? listId : undefined"
      :aria-activedescendant="open && filtered[active] ? optionId(active) : undefined"
      :aria-label="ariaLabel" :aria-labelledby="ariaLabel ? undefined : field?.labelId"
      :aria-describedby="field?.describedBy.value" :aria-invalid="field?.invalid.value || undefined"
      :value="open ? query : selectedLabel" :placeholder="placeholder ?? 'Search…'" autocomplete="off"
      class="min-h-11 w-full rounded-md border border-control bg-input px-3 pr-8 text-sm text-text placeholder:text-muted hover:border-accent focus:border-accent"
      @focus="openList" @click="!open && openList()"
      @input="query = ($event.target as HTMLInputElement).value; open = true; active = 0"
      @keydown="onKeydown" @blur="closeList"
    />
    <Icon name="search" :size="15" class="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 text-muted" />
    <ul v-if="open" :id="listId" ref="list" role="listbox" :aria-labelledby="field?.labelId" :aria-label="ariaLabel || (!field ? 'Options' : undefined)"
      class="absolute z-30 mt-1 max-h-64 w-full overflow-auto rounded-md border border-control bg-surface py-1 shadow-xl">
      <li v-for="(opt, i) in filtered" :id="optionId(i)" :key="opt.value" role="option" :aria-selected="opt.value === model"
        class="flex min-h-11 cursor-pointer items-center px-3 py-2 text-sm"
        :class="i === active ? 'bg-accent-dim text-accent' : 'text-text hover:bg-surface-2'"
        @mousedown.prevent="choose(opt.value)" @mousemove="active = i">{{ opt.label }}</li>
      <li v-if="!filtered.length" role="presentation" class="px-3 py-3 text-sm text-muted"><span role="status">No matches. Try another search.</span></li>
    </ul>
  </div>
</template>
