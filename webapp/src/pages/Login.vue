<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { ApiError } from "@/lib/api";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Icon from "@/components/ui/Icon.vue";
import Input from "@/components/ui/Input.vue";

const auth = useAuthStore();
const router = useRouter();
const route = useRoute();

const name = ref("");
const password = ref("");
const error = ref("");
const busy = ref(false);

async function submit() {
  busy.value = true;
  error.value = "";
  try {
    await auth.login(name.value, password.value);
    const redirect = typeof route.query.redirect === "string" ? route.query.redirect : "/";
    await router.replace(redirect);
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "Login failed";
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="fixed inset-0 flex items-center justify-center bg-bg p-4">
    <form
      class="w-full max-w-sm rounded-md border border-line bg-surface p-6 shadow-2xl"
      @submit.prevent="submit"
    >
      <div class="mb-5 flex items-center gap-3">
        <div class="grid size-10 place-items-center rounded-md border border-accent/30 bg-accent-dim text-sm font-black text-accent">
          SM
        </div>
        <div>
          <h1 class="text-lg font-black tracking-tight">Server Manager</h1>
          <p class="text-sm text-muted">Sign in to manage your race servers</p>
        </div>
      </div>

      <p
        v-if="error"
        class="mb-4 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger"
      >
        {{ error }}
      </p>

      <FormRow label="Username" for-id="name">
        <Input
          id="name"
          v-model="name"
          required
          autocomplete="username"
        />
      </FormRow>
      <FormRow label="Password" for-id="password">
        <Input
          id="password"
          v-model="password"
          type="password"
          required
          autocomplete="current-password"
        />
      </FormRow>

      <Button type="submit" :disabled="busy" class="mt-2 w-full">
        <Icon name="power" :size="16" />
        {{ busy ? "Signing in…" : "Sign in" }}
      </Button>
    </form>
  </div>
</template>
