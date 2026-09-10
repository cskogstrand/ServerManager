<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { ApiError } from "@/lib/api";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Icon from "@/components/ui/Icon.vue";
import Input from "@/components/ui/Input.vue";
import BrandMark from "@/components/ui/BrandMark.vue";

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
  <div class="login-page min-h-dvh bg-bg p-5 pt-24">
    <div class="login-intro"><BrandMark /><p class="mt-8 font-mono text-[11px] tracking-widest text-dim uppercase">Server Manager / Paddock</p><h2 class="mt-5 text-5xl leading-tight font-medium tracking-tighter lg:text-6xl">A better place<br><span class="text-accent">to run your races.</span></h2><p class="mt-6 max-w-sm text-sm leading-relaxed text-muted">Your servers, your setups, your next great race night. Everything in one calm workspace.</p><div class="mt-10 flex gap-3 border-t border-plan-line pt-6 text-xs text-muted"><Icon name="events" :size="17" />Assetto Corsa · Server management</div></div>
    <form
      class="login-form w-full max-w-md rounded-lg border border-line bg-surface p-7 sm:p-9"
      @submit.prevent="submit"
    >
      <div class="mb-8 flex items-center gap-4">
        <BrandMark />
        <div>
          <h1 class="text-2xl font-medium tracking-tight">Welcome to the paddock.</h1>
          <p class="mt-2 text-xs leading-relaxed text-muted">Sign in to manage your race servers.</p>
        </div>
      </div>

      <p
        v-if="error"
        role="alert" class="mb-4 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger"
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

<style scoped>
.login-page { display: flex; align-items: center; justify-content: center; gap: clamp(40px, 8vw, 120px); }
.login-intro { max-width: 540px; }
@media (max-width: 899px) { .login-intro { display: none; } }
</style>
