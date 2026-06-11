<script setup lang="ts">
// Shared layout for the four preset pages: list of presets on the left,
// editor (slot) on the right. Create inline, delete with confirm.
import { ref } from "vue";
import type { DropDownList } from "@/types/generated";
import Button from "@/components/ui/Button.vue";
import Input from "@/components/ui/Input.vue";

defineProps<{
  title: string;
  items: DropDownList[];
  selectedId: number | null;
  busy?: boolean;
}>();

const emit = defineEmits<{
  select: [id: number];
  create: [name: string];
  remove: [id: number];
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
  <h1 class="mb-5 text-xl font-bold">{{ title }}</h1>

  <div class="flex flex-col gap-5 lg:flex-row">
    <aside class="w-full shrink-0 lg:w-64">
      <form class="mb-3 flex gap-2" @submit.prevent="submitCreate">
        <Input v-model="newName" :placeholder="`New ${title.toLowerCase()}…`" />
        <Button type="submit" variant="dark" :disabled="busy">Add</Button>
      </form>

      <ul class="space-y-1">
        <li v-for="item in items" :key="item.id ?? 0" class="group flex items-center">
          <button
            type="button"
            class="min-w-0 flex-1 truncate rounded-md px-3 py-1.5 text-left text-sm"
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
            type="button"
            class="ml-1 hidden px-1 text-dim group-hover:block hover:text-danger"
            :aria-label="`Delete ${item.name}`"
            @click="emit('remove', item.id!)"
          >
            &times;
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
