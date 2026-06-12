<script setup lang="ts">
// Inline preset creation for the event builder: name it, fill the same form
// the full preset page uses, save — then the builder selects it. Reuses the
// per-kind *Fields components so there is no duplicated form logic.
import { ref, computed, watch } from "vue";
import { ApiError } from "@/lib/api";
import { presetFor, presetMeta, type PresetKind } from "@/lib/presetForms";
import { useToastStore } from "@/stores/toast";
import Sheet from "@/components/ui/Sheet.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import ClassFields from "@/components/presets/ClassFields.vue";
import SessionFields from "@/components/presets/SessionFields.vue";
import TimeFields from "@/components/presets/TimeFields.vue";
import DifficultyFields from "@/components/presets/DifficultyFields.vue";

const props = defineProps<{ open: boolean; kind: PresetKind }>();
const emit = defineEmits<{ close: []; created: [id: number] }>();

const toast = useToastStore();

const fieldsComponent = {
  class: ClassFields,
  session: SessionFields,
  time: TimeFields,
  difficulty: DifficultyFields,
} as const;

const name = ref("");
const form = ref<any>(null);
const newId = ref<number | null>(null);
const busy = ref(false);

const meta = computed(() => presetMeta[props.kind]);

watch(
  () => props.open,
  (open) => {
    if (open) {
      name.value = "";
      form.value = null;
      newId.value = null;
    }
  },
);

async function guard(fn: () => Promise<void>) {
  busy.value = true;
  try {
    await fn();
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value = false;
  }
}

function startEditing() {
  const n = name.value.trim();
  if (!n) return;
  void guard(async () => {
    const { resource, prepare } = presetFor(props.kind);
    const id = await resource.create(n);
    const data = await resource.get(id);
    form.value = prepare ? prepare(data) : data;
    if (form.value && (form.value.name === null || form.value.name === undefined || form.value.name === "")) {
      form.value.name = n;
    }
    newId.value = id;
  });
}

function save() {
  if (!form.value || newId.value === null) return;
  void guard(async () => {
    const { resource } = presetFor(props.kind);
    await resource.update(newId.value!, form.value);
    toast.success(`${meta.value.title} created.`);
    emit("created", newId.value!);
    emit("close");
  });
}
</script>

<template>
  <Sheet :open="open" :title="`New ${meta.title.toLowerCase()}`" @close="emit('close')">
    <template v-if="!form">
      <FormRow label="Name" for-id="presetname" hint="Name it, then fill in the details.">
        <Input id="presetname" v-model="name" @keyup.enter="startEditing" />
      </FormRow>
    </template>
    <component :is="fieldsComponent[kind]" v-else v-model="form" />

    <template #footer>
      <Button variant="ghost" @click="emit('close')">Cancel</Button>
      <Button v-if="!form" :disabled="busy || !name.trim()" @click="startEditing">Continue</Button>
      <Button v-else :disabled="busy" @click="save">Save {{ meta.title.toLowerCase() }}</Button>
    </template>
  </Sheet>
</template>
