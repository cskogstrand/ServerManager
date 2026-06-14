<script setup lang="ts">
import { presetResource } from "@/lib/presets";
import { usePresetPage } from "@/lib/usePresetPage";
import { prepareTime } from "@/lib/presetForms";
import { useUnsavedGuard } from "@/lib/useUnsavedGuard";
import type { UserTime } from "@/types/generated";
import PresetShell from "@/components/presets/PresetShell.vue";
import PresetNotices from "@/components/presets/PresetNotices.vue";
import TimeFields from "@/components/presets/TimeFields.vue";
import Button from "@/components/ui/Button.vue";

const resource = presetResource<UserTime>("times", "time");
const page = usePresetPage(resource, prepareTime);
const { form } = page;
useUnsavedGuard(page.isDirty);
</script>

<template>
  <PresetShell
    title="Time & Weather"
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
      <TimeFields v-model="form" />
      <Button type="submit" :disabled="page.busy.value">Save time & weather</Button>
    </form>

    <p v-else class="text-muted">Select a time preset or create a new one.</p>
  </PresetShell>
</template>
