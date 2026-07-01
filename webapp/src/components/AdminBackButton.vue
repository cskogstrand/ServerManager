<script setup lang="ts">
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
</script>

<template>
  <RouterLink to="/admin">
    <Button variant="ghost" size="sm">
      <Icon name="arrowLeft" :size="14" />
      Back to admin
    </Button>
  </RouterLink>
</template>
