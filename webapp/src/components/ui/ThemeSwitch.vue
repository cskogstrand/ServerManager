<script setup lang="ts">
import Icon from "./Icon.vue";
const model = defineModel<"light" | "dark">({ required: true });
</script>

<template>
  <div class="theme-switch" role="group" aria-label="Appearance">
    <button type="button" :aria-pressed="model === 'light'" @click="model = 'light'"><Icon name="sun" :size="13" />Light</button>
    <button type="button" :aria-pressed="model === 'dark'" @click="model = 'dark'"><Icon name="moon" :size="13" />Dark</button>
  </div>
</template>
