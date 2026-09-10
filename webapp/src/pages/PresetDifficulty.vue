<script setup lang="ts">
import { presetResource } from "@/lib/presets";
import { usePresetPage } from "@/lib/usePresetPage";
import type { UserDifficulty } from "@/types/generated";
import PresetShell from "@/components/presets/PresetShell.vue";
import PresetNotices from "@/components/presets/PresetNotices.vue";
import DifficultyFields from "@/components/presets/DifficultyFields.vue";
import Button from "@/components/ui/Button.vue";

const resource = presetResource<UserDifficulty>("difficulties", "difficulty");
const page = usePresetPage(resource);
const { form } = page;
</script>

<template>
  <PresetShell
    title="Difficulty"
    :items="page.items.value"
    :selected-id="page.selectedId.value"
    :busy="page.busy.value"
    :usage="page.usage.value"
    duplicatable
    @select="page.select"
    @create="page.create"
    @remove="page.remove"
    @duplicate="page.duplicate"
  >
    <PresetNotices :notice="page.notice.value" :error="page.error.value" />

    <form v-if="form" class="space-y-4" @submit.prevent="page.save">
      <DifficultyFields v-model="form" />
      <Button type="submit" :disabled="page.busy.value">Save difficulty</Button>
    </form>

    <p v-else class="text-muted">Select a difficulty preset or create a new one.</p>
  </PresetShell>
</template>
