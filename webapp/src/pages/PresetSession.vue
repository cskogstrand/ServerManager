<script setup lang="ts">
import { presetResource } from "@/lib/presets";
import { usePresetPage } from "@/lib/usePresetPage";
import { intToggle } from "@/lib/forms";
import type { UserSession } from "@/types/generated";
import PresetShell from "@/components/presets/PresetShell.vue";
import PresetNotices from "@/components/presets/PresetNotices.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Toggle from "@/components/ui/Toggle.vue";

const resource = presetResource<UserSession>("sessions", "session");
const page = usePresetPage(resource);
const { form } = page;

const booking = intToggle(form, "booking_enabled");
const practice = intToggle(form, "practice_enabled");
const practiceOpen = intToggle(form, "practice_is_open");
const qualify = intToggle(form, "qualify_enabled");
const qualifyOpen = intToggle(form, "qualify_is_open");
const race = intToggle(form, "race_enabled");
const raceOpen = intToggle(form, "race_is_open");
const raceExtraLap = intToggle(form, "race_extra_lap");
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
      <Card title="Identity">
        <FormRow label="Name" for-id="name">
          <Input id="name" v-model="form.name" />
        </FormRow>
      </Card>

      <Card title="Booking">
        <Toggle v-model="booking" label="Enable booking session" />
        <div v-if="booking" class="mt-3 max-w-xs">
          <FormRow label="Duration (minutes)" for-id="booktime">
            <Input id="booktime" v-model="form.booking_time" type="number" :min="1" />
          </FormRow>
        </div>
      </Card>

      <Card title="Practice">
        <Toggle v-model="practice" label="Enable practice session" />
        <div v-if="practice" class="mt-3">
          <div class="max-w-xs">
            <FormRow label="Duration (minutes)" for-id="practime">
              <Input id="practime" v-model="form.practice_time" type="number" :min="1" />
            </FormRow>
          </div>
          <Toggle v-model="practiceOpen" label="Allow joining during session" />
        </div>
      </Card>

      <Card title="Qualifying">
        <Toggle v-model="qualify" label="Enable qualifying session" />
        <div v-if="qualify" class="mt-3">
          <div class="grid gap-x-4 sm:grid-cols-2">
            <FormRow label="Duration (minutes)" for-id="qualtime">
              <Input id="qualtime" v-model="form.qualify_time" type="number" :min="1" />
            </FormRow>
            <FormRow
              label="Lap completion limit (%)"
              for-id="qualwait"
              hint="Slower drivers may finish their lap up to this % of the leader's time"
            >
              <Input id="qualwait" v-model="form.qualify_max_wait_perc" type="number" :min="100" :max="300" />
            </FormRow>
          </div>
          <Toggle v-model="qualifyOpen" label="Allow joining during session" />
        </div>
      </Card>

      <Card title="Race">
        <Toggle v-model="race" label="Enable race session" />
        <div v-if="race" class="mt-3">
          <div class="grid gap-x-4 sm:grid-cols-2">
            <FormRow label="Duration (minutes)" for-id="racetime" hint="Used when the event has no lap count">
              <Input id="racetime" v-model="form.race_time" type="number" :min="1" />
            </FormRow>
            <FormRow label="Overtime (seconds)" for-id="raceover">
              <Input id="raceover" v-model="form.race_over_time" type="number" :min="0" />
            </FormRow>
            <FormRow label="Wait before start (s)" for-id="racewait">
              <Input id="racewait" v-model="form.race_wait_time" type="number" :min="0" />
            </FormRow>
            <FormRow label="Reversed grid positions" for-id="revgrid">
              <Input id="revgrid" v-model="form.reversed_grid_positions" type="number" :min="0" />
            </FormRow>
            <FormRow label="Pit window start (lap/min)" for-id="pitstart">
              <Input id="pitstart" v-model="form.race_pit_window_start" type="number" :min="0" />
            </FormRow>
            <FormRow label="Pit window end (lap/min)" for-id="pitend">
              <Input id="pitend" v-model="form.race_pit_window_end" type="number" :min="0" />
            </FormRow>
          </div>
          <div class="flex flex-wrap gap-x-8 gap-y-2">
            <Toggle v-model="raceOpen" label="Allow joining during session" />
            <Toggle v-model="raceExtraLap" label="Extra lap after timer" />
          </div>
        </div>
      </Card>

      <Button type="submit" :disabled="page.busy.value">Save sessions</Button>
    </form>

    <p v-else class="text-muted">Select a session preset or create a new one.</p>
  </PresetShell>
</template>
