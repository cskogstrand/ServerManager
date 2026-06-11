<script setup lang="ts">
import { presetResource } from "@/lib/presets";
import { usePresetPage } from "@/lib/usePresetPage";
import { intToggle } from "@/lib/forms";
import type { UserDifficulty } from "@/types/generated";
import PresetShell from "@/components/presets/PresetShell.vue";
import PresetNotices from "@/components/presets/PresetNotices.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Toggle from "@/components/ui/Toggle.vue";

const resource = presetResource<UserDifficulty>("difficulties", "difficulty");
const page = usePresetPage(resource);
const { form } = page;

const stability = intToggle(form, "stability_allowed");
const autoclutch = intToggle(form, "autoclutch_allowed");
const tyreBlankets = intToggle(form, "tyre_blankets_allowed");
const virtualMirror = intToggle(form, "force_virtual_mirror");
const gasPenalty = intToggle(form, "race_gas_penality_disabled");
const dynamicTrack = intToggle(form, "dynamic_track");

const assistLevels = [
  { value: 0, label: "Denied" },
  { value: 1, label: "Factory" },
  { value: 2, label: "Forced" },
];
const trackPresets = ["Custom", "Dusty", "Old", "Slow", "Green", "Fast", "Optimum"].map(
  (label, value) => ({ value, label }),
);
</script>

<template>
  <PresetShell
    title="Difficulty"
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

      <Card title="Assists">
        <div class="grid gap-x-4 sm:grid-cols-2">
          <FormRow label="ABS" for-id="abs">
            <Select id="abs" v-model="form.abs_allowed" :options="assistLevels" />
          </FormRow>
          <FormRow label="Traction control" for-id="tc">
            <Select id="tc" v-model="form.tc_allowed" :options="assistLevels" />
          </FormRow>
        </div>
        <div class="grid gap-2 pt-1 sm:grid-cols-2">
          <Toggle v-model="stability" label="Stability control" />
          <Toggle v-model="autoclutch" label="Auto clutch" />
          <Toggle v-model="tyreBlankets" label="Tyre blankets" />
          <Toggle v-model="virtualMirror" label="Force virtual mirror" />
        </div>
      </Card>

      <Card title="Realism">
        <div class="grid gap-x-4 sm:grid-cols-3">
          <FormRow label="Fuel rate (%)" for-id="fuel">
            <Input id="fuel" v-model="form.fuel_rate" type="number" :min="0" :max="500" />
          </FormRow>
          <FormRow label="Damage rate (%)" for-id="damage">
            <Input id="damage" v-model="form.damage_multiplier" type="number" :min="0" :max="100" />
          </FormRow>
          <FormRow label="Tyre wear (%)" for-id="wear">
            <Input id="wear" v-model="form.tyre_wear_rate" type="number" :min="0" :max="500" />
          </FormRow>
          <FormRow label="Allowed tyres out" for-id="tyresout">
            <Select
              id="tyresout"
              v-model="form.allowed_tyres_out"
              :options="[0, 1, 2, 3, 4].map((n) => ({ value: n, label: String(n) }))"
            />
          </FormRow>
          <FormRow label="Max ballast (kg)" for-id="ballast">
            <Input id="ballast" v-model="form.max_ballast_kg" type="number" :min="0" />
          </FormRow>
          <FormRow label="Jump start rule" for-id="startrule">
            <Select
              id="startrule"
              v-model="form.start_rule"
              :options="[
                { value: 0, label: 'Car locked' },
                { value: 1, label: 'Teleport to pit' },
                { value: 2, label: 'Drive through' },
              ]"
            />
          </FormRow>
          <FormRow label="Max collisions per km" for-id="contacts">
            <Input id="contacts" v-model="form.max_contacts_per_km" type="number" :min="-1" />
          </FormRow>
        </div>
        <Toggle v-model="gasPenalty" label="Disable gas-cut penalty" />
      </Card>

      <Card title="Dynamic track">
        <Toggle v-model="dynamicTrack" label="Enable dynamic track grip" />
        <div v-if="dynamicTrack" class="mt-3 grid gap-x-4 sm:grid-cols-2">
          <FormRow label="Preset" for-id="dtpreset">
            <Select id="dtpreset" v-model="form.dynamic_track_preset" :options="trackPresets" />
          </FormRow>
          <FormRow label="Start grip (%)" for-id="dtstart">
            <Input id="dtstart" v-model="form.session_start" type="number" :min="0" :max="100" />
          </FormRow>
          <FormRow label="Randomness" for-id="dtrandom">
            <Input id="dtrandom" v-model="form.randomness" type="number" :min="0" />
          </FormRow>
          <FormRow label="Transfer grip (%)" for-id="dttransfer">
            <Input id="dttransfer" v-model="form.session_transfer" type="number" :min="0" :max="100" />
          </FormRow>
          <FormRow label="Laps to improve grip" for-id="dtlaps">
            <Input id="dtlaps" v-model="form.lap_gain" type="number" :min="0" />
          </FormRow>
        </div>
      </Card>

      <Card title="Voting & blacklist">
        <div class="grid gap-x-4 sm:grid-cols-2">
          <FormRow label="Kick vote quorum (%)" for-id="kickq">
            <Input id="kickq" v-model="form.kick_quorum" type="number" :min="0" :max="100" />
          </FormRow>
          <FormRow label="Session vote quorum (%)" for-id="voteq">
            <Input id="voteq" v-model="form.voting_quorum" type="number" :min="0" :max="100" />
          </FormRow>
          <FormRow label="Vote duration (s)" for-id="votedur">
            <Input id="votedur" v-model="form.vote_duration" type="number" :min="0" />
          </FormRow>
          <FormRow label="Blacklist mode" for-id="blacklist">
            <Select
              id="blacklist"
              v-model="form.blacklist_mode"
              :options="[
                { value: 0, label: 'Kick player' },
                { value: 1, label: 'Kick until restart' },
              ]"
            />
          </FormRow>
        </div>
      </Card>

      <Button type="submit" :disabled="page.busy.value">Save difficulty</Button>
    </form>

    <p v-else class="text-muted">Select a difficulty preset or create a new one.</p>
  </PresetShell>
</template>
