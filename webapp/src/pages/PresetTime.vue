<script setup lang="ts">
import { onMounted } from "vue";
import { presetResource } from "@/lib/presets";
import { usePresetPage } from "@/lib/usePresetPage";
import { intToggle } from "@/lib/forms";
import { useContentStore } from "@/stores/content";
import type { UserTime, UserTimeWeather } from "@/types/generated";
import PresetShell from "@/components/presets/PresetShell.vue";
import PresetNotices from "@/components/presets/PresetNotices.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Toggle from "@/components/ui/Toggle.vue";

const content = useContentStore();
onMounted(() => void content.load());

const resource = presetResource<UserTime>("times", "time");

// The GET payload serializes weather numbers as strings (legacy ",string"
// json tags) — normalize to real numbers so number inputs and the PUT DTO
// behave.
const numericWeatherFields = [
  "base_temperature_ambient",
  "base_temperature_road",
  "variation_ambient",
  "variation_road",
  "wind_base_speed_min",
  "wind_base_speed_max",
  "wind_base_direction",
  "wind_variation_direction",
  "csp_time_of_day_multi",
] as const;

const page = usePresetPage(resource, (form) => {
  if (!form.weathers || form.weathers.length === 0) {
    form.weathers = [emptyWeather()];
  }
  for (const w of form.weathers) {
    for (const key of numericWeatherFields) {
      const v = (w as any)[key];
      if (typeof v === "string") (w as any)[key] = v === "" ? null : Number(v);
    }
  }
  return form;
});
const { form } = page;

const cspEnabled = intToggle(form, "csp_enabled");

function emptyWeather(): UserTimeWeather {
  return {
    graphics: undefined,
    base_temperature_ambient: 20,
    base_temperature_road: 7,
    variation_ambient: 2,
    variation_road: 2,
    wind_base_speed_min: 0,
    wind_base_speed_max: 10,
    wind_base_direction: 0,
    wind_variation_direction: 0,
  } as UserTimeWeather;
}

function addWeather() {
  form.value?.weathers?.push(emptyWeather());
}

function removeWeather(index: number) {
  form.value?.weathers?.splice(index, 1);
}
</script>

<template>
  <PresetShell
    title="Time & Weather"
    :items="page.items.value"
    :selected-id="page.selectedId.value"
    :busy="page.busy.value"
    @select="page.select"
    @create="page.create"
    @remove="page.remove"
  >
    <PresetNotices :notice="page.notice.value" :error="page.error.value" />

    <form v-if="form" class="space-y-4" @submit.prevent="page.save">
      <Card title="Time of day">
        <FormRow label="Name" for-id="name">
          <Input id="name" v-model="form.name" />
        </FormRow>
        <Toggle v-model="cspEnabled" label="CSP weather (per-panel time, 24h, dates)" />
        <div v-if="!cspEnabled" class="mt-3 grid max-w-md gap-x-4 sm:grid-cols-2">
          <FormRow label="Time of day" for-id="time" hint="08:00 – 18:00 (vanilla limit)">
            <Input id="time" v-model="form.time" type="time" />
          </FormRow>
          <FormRow label="Time multiplier" for-id="multi">
            <Input id="multi" v-model="form.time_of_day_multi" type="number" :min="1" :max="10" />
          </FormRow>
        </div>
      </Card>

      <Card v-for="(weather, i) in form.weathers" :key="i" :title="`Weather panel ${i + 1}`">
        <template #actions>
          <Button
            v-if="(form.weathers?.length ?? 0) > 1"
            variant="ghost"
            size="sm"
            @click="removeWeather(i)"
          >
            Remove
          </Button>
        </template>

        <div class="grid gap-x-4 sm:grid-cols-2">
          <FormRow label="Weather">
            <Select
              v-model="weather.graphics"
              :options="content.weathers.map((w) => ({ value: w.key ?? '', label: w.name ?? w.key ?? '' }))"
            />
          </FormRow>
        </div>

        <div v-if="cspEnabled" class="grid gap-x-4 sm:grid-cols-3">
          <FormRow label="Time of day (CSP)">
            <Input v-model="weather.csp_time" type="time" />
          </FormRow>
          <FormRow label="Multiplier (CSP)">
            <Input v-model="weather.csp_time_of_day_multi" type="number" :min="0" :max="60" />
          </FormRow>
          <FormRow label="Date (optional)">
            <Input v-model="weather.csp_date" type="date" />
          </FormRow>
        </div>

        <div class="grid gap-x-4 sm:grid-cols-2 lg:grid-cols-4">
          <FormRow label="Ambient temp (°C)">
            <Input v-model="weather.base_temperature_ambient" type="number" :min="-20" :max="50" />
          </FormRow>
          <FormRow label="Road temp (± ambient)">
            <Input v-model="weather.base_temperature_road" type="number" :min="-20" :max="50" />
          </FormRow>
          <FormRow label="Ambient variation (±)">
            <Input v-model="weather.variation_ambient" type="number" :min="0" :max="20" />
          </FormRow>
          <FormRow label="Road variation (±)">
            <Input v-model="weather.variation_road" type="number" :min="0" :max="20" />
          </FormRow>
          <FormRow label="Wind min (km/h)">
            <Input v-model="weather.wind_base_speed_min" type="number" :min="0" :max="40" />
          </FormRow>
          <FormRow label="Wind max (km/h)">
            <Input v-model="weather.wind_base_speed_max" type="number" :min="0" :max="40" />
          </FormRow>
          <FormRow label="Wind direction (°)">
            <Input v-model="weather.wind_base_direction" type="number" :min="0" :max="359" />
          </FormRow>
          <FormRow label="Direction variation (°)">
            <Input v-model="weather.wind_variation_direction" type="number" :min="0" :max="359" />
          </FormRow>
        </div>
      </Card>

      <div class="flex gap-2">
        <Button variant="dark" @click="addWeather">Add weather panel</Button>
        <Button type="submit" :disabled="page.busy.value">Save time & weather</Button>
      </div>
    </form>

    <p v-else class="text-muted">Select a time preset or create a new one.</p>
  </PresetShell>
</template>
