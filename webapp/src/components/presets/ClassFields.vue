<script setup lang="ts">
// Car-class preset form body. Shared by the Car Classes page and the inline
// sheet. Caller is responsible for prepareClass() before binding.
import { onMounted } from "vue";
import { useContentStore } from "@/stores/content";
import type { UserClass, UserClassEntry } from "@/types/generated";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Combobox from "@/components/ui/Combobox.vue";
import Icon from "@/components/ui/Icon.vue";

const form = defineModel<UserClass | null>({ required: true });

const content = useContentStore();
onMounted(() => void content.load());

function addEntry() {
  form.value?.entries?.push({ cache_car_key: undefined, skin_key: "", count: 1 } as UserClassEntry);
}
function removeEntry(index: number) {
  form.value?.entries?.splice(index, 1);
}
function move(index: number, delta: number) {
  const entries = form.value?.entries;
  if (!entries) return;
  const target = index + delta;
  if (target < 0 || target >= entries.length) return;
  [entries[index], entries[target]] = [entries[target], entries[index]];
}
function skinsFor(carKey: string | undefined) {
  return content.carByKey(carKey)?.skins ?? [];
}
function onCarChange(entry: UserClassEntry) {
  entry.skin_key = skinsFor(entry.cache_car_key)[0]?.key ?? "";
}
function randomSkin(entry: UserClassEntry) {
  const skins = skinsFor(entry.cache_car_key);
  if (skins.length === 0) return;
  entry.skin_key = skins[Math.floor(Math.random() * skins.length)].key;
}
function previewUrl(entry: UserClassEntry) {
  if (!entry.cache_car_key || !entry.skin_key) return "";
  return `/api/car/image/${entry.cache_car_key}/${entry.skin_key}`;
}
function totalSlots(): number {
  return form.value?.entries?.reduce((sum, e) => sum + (e.count ?? 1), 0) ?? 0;
}
</script>

<template>
  <div v-if="form" class="space-y-4">
    <Card title="Identity">
      <FormRow label="Name" for-id="name">
        <Input id="name" v-model="form.name" />
      </FormRow>
    </Card>

    <Card :title="`Entries — ${totalSlots()} grid slot${totalSlots() === 1 ? '' : 's'}`">
      <div
        v-for="(entry, i) in form.entries"
        :key="i"
        class="mb-3 flex flex-wrap items-start gap-3 rounded-md border border-line bg-surface-2/50 p-3"
      >
        <img
          v-if="previewUrl(entry)"
          :src="previewUrl(entry)"
          alt=""
          class="h-16 w-28 rounded-sm border border-line object-cover"
        />
        <div v-else class="flex h-16 w-28 items-center justify-center rounded-sm border border-line text-xs text-dim">
          no preview
        </div>

        <div class="grid min-w-0 flex-1 gap-x-4 sm:grid-cols-[2fr_2fr_auto]">
          <FormRow label="Car">
            <Combobox
              v-model="entry.cache_car_key"
              placeholder="Search cars…"
              :options="content.cars.map((c) => ({ value: c.key ?? '', label: c.name ?? c.key ?? '' }))"
              @update:model-value="onCarChange(entry)"
            />
          </FormRow>
          <FormRow label="Skin">
            <div class="flex gap-1">
              <Select
                v-model="entry.skin_key"
                class="flex-1"
                :options="skinsFor(entry.cache_car_key).map((s) => ({ value: s.key, label: s.name || s.key }))"
              />
              <Button variant="dark" size="sm" title="Random skin" aria-label="Random skin" @click="randomSkin(entry)">
                <Icon name="shuffle" :size="14" />
              </Button>
            </div>
          </FormRow>
          <FormRow label="Cars" hint="Grid slots for this entry">
            <Input v-model="entry.count" type="number" :min="1" :max="64" class="w-20" />
          </FormRow>
        </div>

        <div class="flex flex-col gap-1">
          <Button variant="dark" size="sm" :disabled="i === 0" aria-label="Move up" @click="move(i, -1)">
            <Icon name="arrowUp" :size="14" />
          </Button>
          <Button
            variant="dark"
            size="sm"
            :disabled="i === (form.entries?.length ?? 0) - 1"
            aria-label="Move down"
            @click="move(i, 1)"
          >
            <Icon name="arrowDown" :size="14" />
          </Button>
          <Button variant="ghost" size="sm" aria-label="Remove entry" @click="removeEntry(i)">
            <Icon name="x" :size="14" />
          </Button>
        </div>
      </div>

      <Button variant="dark" @click="addEntry">
        <Icon name="plus" :size="15" />
        Add car
      </Button>
    </Card>
  </div>
</template>
