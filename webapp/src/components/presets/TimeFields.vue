<script setup lang="ts">
// Time & weather preset form body. Shared by the Time page and the inline
// sheet. Caller is responsible for prepareTime() before binding.
import { onMounted } from "vue";
import { intToggle } from "@/lib/forms";
import { emptyWeather } from "@/lib/presetForms";
import { useContentStore } from "@/stores/content";
import type { UserTime } from "@/types/generated";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Toggle from "@/components/ui/Toggle.vue";

const form = defineModel<UserTime | null>({ required: true });

const content = useContentStore();
onMounted(() => void content.load());

const cspEnabled = intToggle(form, "csp_enabled");

function addWeather() {
  form.value?.weathers?.push(emptyWeather());
}
function removeWeather(index: number) {
  form.value?.weathers?.splice(index, 1);
}
</script>

<template>
  <div v-if="form" class="space-y-4">
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
        <Button v-if="(form.weathers?.length ?? 0) > 1" variant="ghost" size="sm" @click="removeWeather(i)">
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

    <Button variant="dark" @click="addWeather">Add weather panel</Button>
  </div>
</template>
