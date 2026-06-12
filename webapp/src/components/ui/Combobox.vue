<script setup lang="ts">
// Searchable single-select. Drop-in for Select when the option list can grow
// (cars, presets). Shows the selected label; typing filters; click/Enter picks.
import { computed, ref } from "vue";
import Icon from "@/components/ui/Icon.vue";

const props = defineProps<{
  id?: string;
  options: { value: string | number; label: string }[];
  placeholder?: string;
}>();

const model = defineModel<string | number | null>();

const open = ref(false);
const query = ref("");
const active = ref(0);

const selectedLabel = computed(() => props.options.find((o) => o.value === model.value)?.label ?? "");

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase();
  if (!q) return props.options;
  return props.options.filter((o) => o.label.toLowerCase().includes(q));
});

function openList() {
  open.value = true;
  query.value = "";
  active.value = Math.max(
    0,
    filtered.value.findIndex((o) => o.value === model.value),
  );
}

function choose(value: string | number) {
  model.value = value;
  open.value = false;
  query.value = "";
}

function onKeydown(e: KeyboardEvent) {
  if (!open.value && (e.key === "ArrowDown" || e.key === "Enter")) {
    openList();
    return;
  }
  if (e.key === "ArrowDown") {
    active.value = Math.min(active.value + 1, filtered.value.length - 1);
    e.preventDefault();
  } else if (e.key === "ArrowUp") {
    active.value = Math.max(active.value - 1, 0);
    e.preventDefault();
  } else if (e.key === "Enter") {
    const opt = filtered.value[active.value];
    if (opt) choose(opt.value);
    e.preventDefault();
  } else if (e.key === "Escape") {
    open.value = false;
  }
}
</script>

<template>
  <div class="relative">
    <input
      :id="id"
      :value="open ? query : selectedLabel"
      :placeholder="placeholder ?? 'Search…'"
      autocomplete="off"
      class="min-h-9 w-full cursor-text rounded-md border border-line bg-surface-2 px-3 pr-8 text-sm text-text outline-none transition-colors duration-200 hover:border-line-hi focus:border-accent focus:bg-surface-3 focus:ring-2 focus:ring-accent/20"
      @focus="openList"
      @input="query = ($event.target as HTMLInputElement).value; open = true; active = 0"
      @keydown="onKeydown"
      @blur="open = false"
    />
    <Icon name="search" :size="14" class="pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2 text-dim" />

    <ul
      v-if="open && filtered.length"
      class="absolute z-30 mt-1 max-h-56 w-full overflow-auto rounded-md border border-line bg-surface shadow-xl"
    >
      <li
        v-for="(opt, i) in filtered"
        :key="opt.value"
        class="cursor-pointer px-3 py-1.5 text-sm"
        :class="[
          i === active ? 'bg-accent-dim text-accent' : 'text-text hover:bg-surface-2',
          opt.value === model ? 'font-semibold' : '',
        ]"
        @mousedown.prevent="choose(opt.value)"
        @mousemove="active = i"
      >
        {{ opt.label }}
      </li>
    </ul>
    <div
      v-else-if="open"
      class="absolute z-30 mt-1 w-full rounded-md border border-line bg-surface px-3 py-2 text-sm text-dim shadow-xl"
    >
      No matches
    </div>
  </div>
</template>
