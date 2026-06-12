<script setup lang="ts">
// Shimmer placeholder. Pass through height/width via class.
defineProps<{ class?: string }>();
</script>

<template>
  <div class="skeleton rounded-md bg-surface-2" />
</template>

<style scoped>
.skeleton {
  position: relative;
  overflow: hidden;
}
.skeleton::after {
  content: "";
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.06), transparent);
  animation: shimmer 1.4s infinite;
}
@keyframes shimmer {
  100% {
    transform: translateX(100%);
  }
}
</style>
