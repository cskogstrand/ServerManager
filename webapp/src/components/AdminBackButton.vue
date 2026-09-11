<script setup lang="ts">
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
</script>

<template>
  <RouterLink to="/garage/advanced">
    <Button variant="ghost" size="sm">
      <Icon name="arrowLeft" :size="14" />
      Back to Advanced
    </Button>
  </RouterLink>
</template>
