<script setup lang="ts">
import { presetResource } from "@/lib/presets";
import { usePresetPage } from "@/lib/usePresetPage";
import { prepareClass } from "@/lib/presetForms";
import type { UserClass } from "@/types/generated";
import PresetShell from "@/components/presets/PresetShell.vue";
import PresetNotices from "@/components/presets/PresetNotices.vue";
import ClassFields from "@/components/presets/ClassFields.vue";
import Button from "@/components/ui/Button.vue";

const resource = presetResource<UserClass>("classes", "class");
const page = usePresetPage(resource, prepareClass);
const { form } = page;
</script>

<template>
  <PresetShell
    title="Car Classes"
    :items="page.items.value"
    :selected-id="page.selectedId.value"
    :busy="page.busy.value"
    @select="page.select"
    @create="page.create"
    @remove="page.remove"
  >
    <PresetNotices :notice="page.notice.value" :error="page.error.value" />

    <form v-if="form" class="space-y-4" @submit.prevent="page.save">
      <ClassFields v-model="form" />
      <Button type="submit" :disabled="page.busy.value">Save class</Button>
    </form>

    <p v-else class="text-muted">Select a car class or create a new one.</p>
  </PresetShell>
</template>
