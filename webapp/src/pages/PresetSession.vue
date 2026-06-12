<script setup lang="ts">
import { presetResource } from "@/lib/presets";
import { usePresetPage } from "@/lib/usePresetPage";
import type { UserSession } from "@/types/generated";
import PresetShell from "@/components/presets/PresetShell.vue";
import PresetNotices from "@/components/presets/PresetNotices.vue";
import SessionFields from "@/components/presets/SessionFields.vue";
import Button from "@/components/ui/Button.vue";

const resource = presetResource<UserSession>("sessions", "session");
const page = usePresetPage(resource);
const { form } = page;
</script>

<template>
  <PresetShell
    title="Sessions"
    :items="page.items.value"
    :selected-id="page.selectedId.value"
    :busy="page.busy.value"
    @select="page.select"
    @create="page.create"
    @remove="page.remove"
  >
    <PresetNotices :notice="page.notice.value" :error="page.error.value" />

    <form v-if="form" class="space-y-4" @submit.prevent="page.save">
      <SessionFields v-model="form" />
      <Button type="submit" :disabled="page.busy.value">Save sessions</Button>
    </form>

    <p v-else class="text-muted">Select a session preset or create a new one.</p>
  </PresetShell>
</template>
