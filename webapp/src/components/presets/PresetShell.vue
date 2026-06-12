<script setup lang="ts">
// Shared layout for the four preset pages: list of presets on the left,
// editor (slot) on the right. Create inline, delete with confirm.
import { ref } from "vue";
import type { DropDownList } from "@/types/generated";
import Button from "@/components/ui/Button.vue";
import Input from "@/components/ui/Input.vue";
import Icon from "@/components/ui/Icon.vue";
import PageHeader from "@/components/ui/PageHeader.vue";

defineProps<{
  title: string;
  subtitle?: string;
  icon?: string;
  items: DropDownList[];
  selectedId: number | null;
  busy?: boolean;
  duplicatable?: boolean;
}>();

const emit = defineEmits<{
  select: [id: number];
  create: [name: string];
  remove: [id: number];
  duplicate: [id: number];
}>();

const newName = ref("");

function submitCreate() {
  const name = newName.value.trim();
  if (!name) return;
  emit("create", name);
  newName.value = "";
}
</script>

<template>
  <PageHeader
    :title="title"
    :subtitle="subtitle ?? 'Create, select, and maintain reusable server presets.'"
    :icon="icon ?? 'settings'"
  />

  <div class="flex flex-col gap-5 lg:flex-row">
    <aside class="w-full shrink-0 rounded-md border border-line bg-surface p-3 lg:w-72">
      <form class="mb-3 flex gap-2" @submit.prevent="submitCreate">
        <Input v-model="newName" :placeholder="`New ${title.toLowerCase()}…`" />
        <Button type="submit" variant="dark" :disabled="busy" aria-label="Add">
          <Icon name="plus" :size="15" />
        </Button>
      </form>

      <ul class="space-y-1">
        <li v-for="item in items" :key="item.id ?? 0" class="group flex items-center">
          <button
            type="button"
            class="min-h-9 min-w-0 flex-1 cursor-pointer truncate rounded-md px-3 text-left text-sm font-medium transition-colors"
            :class="
              item.id === selectedId
                ? 'bg-accent-dim text-accent'
                : 'text-muted hover:bg-surface-2 hover:text-text'
            "
            @click="emit('select', item.id!)"
          >
            {{ item.name }}
          </button>
          <button
            v-if="duplicatable"
            type="button"
            class="ml-1 hidden size-8 cursor-pointer place-items-center rounded-md text-dim transition-colors group-hover:grid hover:bg-surface-2 hover:text-text"
            :aria-label="`Duplicate ${item.name}`"
            @click="emit('duplicate', item.id!)"
          >
            <Icon name="copy" :size="14" />
          </button>
          <button
            type="button"
            class="ml-1 hidden size-8 cursor-pointer place-items-center rounded-md text-dim transition-colors group-hover:grid hover:bg-danger-glow hover:text-danger"
            :aria-label="`Delete ${item.name}`"
            @click="emit('remove', item.id!)"
          >
            <Icon name="trash" :size="14" />
          </button>
        </li>
      </ul>
      <p v-if="items.length === 0" class="px-1 text-sm text-dim">Nothing here yet — add one above.</p>
    </aside>

    <div class="min-w-0 flex-1">
      <slot />
    </div>
  </div>
</template>
