<script setup lang="ts">
import { onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Icon from "@/components/ui/Icon.vue";
import PageHeader from "@/components/ui/PageHeader.vue";

const auth = useAuthStore();

const measurementUnit = ref<number>(0);
const tempUnit = ref<number>(0);
const newPassword = ref("");
const confirmPassword = ref("");
const busy = ref(false);
const notice = ref("");
const error = ref("");

onMounted(async () => {
  await auth.ensureChecked();
  measurementUnit.value = auth.user?.measurement_unit ?? 0;
  tempUnit.value = auth.user?.temp_unit ?? 0;
});

async function save() {
  error.value = "";
  notice.value = "";
  if (newPassword.value && newPassword.value !== confirmPassword.value) {
    error.value = "Passwords do not match.";
    return;
  }

  busy.value = true;
  try {
    await api.put("/api/user", {
      measurement_unit: measurementUnit.value,
      temp_unit: tempUnit.value,
      ...(newPassword.value ? { password: newPassword.value } : {}),
    });
    await auth.refresh();
    notice.value = newPassword.value ? "Preferences saved. Password updated." : "Preferences saved.";
    newPassword.value = "";
    confirmPassword.value = "";
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <PageHeader
    title="Preferences"
    subtitle="Personal units and account password settings."
    icon="user"
  />

  <p v-if="notice" class="mb-4 rounded-md border border-ok/40 bg-ok-glow px-3 py-2 text-sm text-ok">
    {{ notice }}
  </p>
  <p v-if="error" class="mb-4 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger">
    {{ error }}
  </p>

  <form class="max-w-xl space-y-4" @submit.prevent="save">
    <Card title="Units">
      <FormRow label="Measurement units" for-id="munit">
        <Select
          id="munit"
          v-model="measurementUnit"
          :options="[
            { value: 0, label: 'KM, KM/h' },
            { value: 1, label: 'Miles, MPH' },
          ]"
        />
      </FormRow>
      <FormRow label="Temperature units" for-id="tunit">
        <Select
          id="tunit"
          v-model="tempUnit"
          :options="[
            { value: 0, label: 'Celsius' },
            { value: 1, label: 'Fahrenheit' },
          ]"
        />
      </FormRow>
    </Card>

    <Card title="Change password">
      <FormRow label="New password" for-id="newpw" hint="Leave empty to keep the current password">
        <Input id="newpw" v-model="newPassword" type="password" autocomplete="new-password" />
      </FormRow>
      <FormRow label="Confirm new password" for-id="confirmpw">
        <Input id="confirmpw" v-model="confirmPassword" type="password" autocomplete="new-password" />
      </FormRow>
    </Card>

    <Button type="submit" :disabled="busy">
      <Icon name="check" :size="15" />
      {{ busy ? "Saving…" : "Save preferences" }}
    </Button>
  </form>
</template>
