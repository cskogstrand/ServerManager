<script setup lang="ts">
defineProps<{ notice?: string; error?: string }>();
</script>

<template>
  <p v-if="notice" class="mb-4 rounded-md border border-ok/40 bg-ok-glow px-3 py-2 text-sm text-ok">
    {{ notice }}
  </p>
  <p v-if="error" class="mb-4 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger">
    {{ error }}
  </p>
</template>
