<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { ApiError } from "@/lib/api";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";

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
      class="w-full max-w-sm rounded-lg border border-line bg-surface p-6"
      @submit.prevent="submit"
    >
      <h1 class="mb-1 text-lg font-bold">Server Manager</h1>
      <p class="mb-5 text-sm text-muted">Sign in to manage your servers</p>

      <p
        v-if="error"
        class="mb-4 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger"
      >
        {{ error }}
      </p>

      <FormRow label="Username" for-id="name">
        <input
          id="name"
          v-model="name"
          required
          autocomplete="username"
          class="w-full rounded-md border border-line bg-surface-2 px-3 py-1.5 text-sm outline-none focus:border-accent"
        />
      </FormRow>
      <FormRow label="Password" for-id="password">
        <input
          id="password"
          v-model="password"
          type="password"
          required
          autocomplete="current-password"
          class="w-full rounded-md border border-line bg-surface-2 px-3 py-1.5 text-sm outline-none focus:border-accent"
        />
      </FormRow>

      <Button type="submit" :disabled="busy" class="mt-2 w-full">
        {{ busy ? "Signing in…" : "Sign in" }}
      </Button>
    </form>
  </div>
</template>
