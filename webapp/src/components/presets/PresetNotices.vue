<script setup lang="ts">
import Icon from "@/components/ui/Icon.vue";

defineProps<{ notice?: string; error?: string }>();
</script>

<template>
  <p v-if="notice" class="mb-4 flex gap-2 rounded-md border border-ok/40 bg-ok-glow px-3 py-2 text-sm text-ok">
    <Icon name="check" :size="16" class="mt-0.5 shrink-0" />
    {{ notice }}
  </p>
  <p v-if="error" class="mb-4 flex gap-2 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger">
    <Icon name="info" :size="16" class="mt-0.5 shrink-0" />
    {{ error }}
  </p>
</template>
