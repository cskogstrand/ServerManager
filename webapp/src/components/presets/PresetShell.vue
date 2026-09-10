<script setup lang="ts">
// Shared layout for the four preset pages: searchable list of presets on the
// left (each with a "used by N" count), editor (slot) on the right. Create
// inline, duplicate, delete with confirm. When the selected preset is used by
// existing events, a clone-before-edit banner offers to duplicate first so
// changes don't silently rewrite every event that shares it.
import { computed, ref } from "vue";
import { useQueryParam } from "@/lib/useQueryParam";
import type { DropDownList } from "@/types/generated";
import Button from "@/components/ui/Button.vue";
import Input from "@/components/ui/Input.vue";
import Icon from "@/components/ui/Icon.vue";
import PageHeader from "@/components/ui/PageHeader.vue";

const props = defineProps<{
  title: string;
  subtitle?: string;
  icon?: string;
  items: DropDownList[];
  selectedId: number | null;
  busy?: boolean;
  duplicatable?: boolean;
  // { presetId: eventCount } — how many events use each preset.
  usage?: Record<string, number>;
}>();

const emit = defineEmits<{
  select: [id: number];
  create: [name: string];
  remove: [id: number];
  duplicate: [id: number];
}>();

const newName = ref("");
const search = useQueryParam("q", "");

function submitCreate() {
  const name = newName.value.trim();
  if (!name) return;
  emit("create", name);
  newName.value = "";
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase();
  if (!q) return props.items;
  return props.items.filter((i) => (i.name ?? "").toLowerCase().includes(q));
});

function uses(id: number | null | undefined): number {
  if (id == null) return 0;
  return props.usage?.[String(id)] ?? 0;
}

const selectedUses = computed(() => uses(props.selectedId));
</script>

<template>
  <PageHeader
    :title="title"
    :subtitle="subtitle ?? 'Create, select, and maintain reusable server presets.'"
    :icon="icon ?? 'settings'"
  >
    <template #prefix>
      <RouterLink to="/presets">
        <Button variant="ghost" size="sm">
          <Icon name="arrowLeft" :size="14" />
          Back to templates
        </Button>
      </RouterLink>
    </template>
  </PageHeader>

  <div class="flex flex-col gap-5 lg:flex-row">
    <aside class="w-full shrink-0 rounded-md border border-line bg-surface p-3 lg:w-72">
      <form class="mb-2 flex gap-2" @submit.prevent="submitCreate">
        <Input v-model="newName" :aria-label="`New ${title.toLowerCase()} name`" :placeholder="`New ${title.toLowerCase()}…`" />
        <Button type="submit" variant="dark" :disabled="busy || !newName.trim()" aria-label="Create preset">
          <Icon name="plus" :size="15" />
        </Button>
      </form>

      <div v-if="items.length > 6 || search" class="relative mb-2">
        <Icon name="search" :size="14" class="absolute top-1/2 left-2.5 -translate-y-1/2 text-dim" />
        <Input v-model="search" :aria-label="`Search ${title.toLowerCase()}`" :placeholder="`Search ${title.toLowerCase()}…`" class="!pl-8" />
      </div>

      <ul class="space-y-1">
        <li v-for="item in filtered" :key="item.id ?? 0" class="group flex items-center">
          <button
            type="button"
            class="flex min-h-11 min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-md px-3 text-left text-sm font-medium transition-colors"
            :class="
              item.id === selectedId
                ? 'bg-accent-dim text-accent'
                : 'text-muted hover:bg-surface-2 hover:text-text'
            "
            :disabled="busy" :aria-pressed="item.id === selectedId" @click="emit('select', item.id!)"
          >
            <span class="min-w-0 flex-1 truncate">{{ item.name }}</span>
            <span
              v-if="uses(item.id)"
              class="shrink-0 rounded-full bg-surface-3 px-1.5 text-xs text-dim"
              :title="`Used by ${uses(item.id)} event${uses(item.id) === 1 ? '' : 's'}`"
            >
              {{ uses(item.id) }}
            </span>
          </button>
          <button
            v-if="duplicatable"
            type="button"
            class="ml-1 grid size-11 shrink-0 cursor-pointer place-items-center rounded-md text-dim transition-colors hover:bg-surface-2 hover:text-text"
            :disabled="busy" :aria-label="`Duplicate ${item.name}`"
            @click="emit('duplicate', item.id!)"
          >
            <Icon name="copy" :size="14" />
          </button>
          <button
            type="button"
            class="ml-1 grid size-11 shrink-0 cursor-pointer place-items-center rounded-md text-dim transition-colors hover:bg-danger-glow hover:text-danger"
            :disabled="busy" :aria-label="`Delete ${item.name}`"
            @click="emit('remove', item.id!)"
          >
            <Icon name="trash" :size="14" />
          </button>
        </li>
      </ul>
      <p v-if="items.length === 0" class="px-1 text-sm text-dim">Nothing here yet — add one above.</p>
      <p v-else-if="filtered.length === 0" class="px-1 text-sm text-dim">No matches for “{{ search }}”. <button type="button" class="min-h-11 text-accent underline" @click="search = ''">Clear search</button></p>
    </aside>

    <div class="min-w-0 flex-1">
      <!-- Clone-before-edit: editing a shared preset rewrites it for every event -->
      <div
        v-if="duplicatable && selectedId !== null && selectedUses > 0"
        class="mb-3 flex flex-wrap items-center gap-2 rounded-md border border-warn/40 bg-warn-glow px-3 py-2 text-sm"
      >
        <Icon name="alert" :size="16" class="shrink-0 text-warn" />
        <span class="text-muted">
          Used by <span class="font-semibold text-text">{{ selectedUses }}</span>
          event{{ selectedUses === 1 ? "" : "s" }} — saving changes them all.
        </span>
        <Button variant="dark" size="sm" class="ml-auto" :disabled="busy" @click="emit('duplicate', selectedId)">
          <Icon name="copy" :size="14" />
          Duplicate first
        </Button>
      </div>

      <slot />
    </div>
  </div>
</template>
