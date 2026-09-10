<script setup lang="ts">
import { useRoute } from "vue-router";
import EmptyState from "@/components/ui/EmptyState.vue";
const route = useRoute();
</script>
<template>
  <EmptyState icon="lock" :title="route.query.role === 'admin' ? 'Administrator access required' : 'Operator access required'"
    message="Your account cannot open this page. Ask an administrator for help, or return to the server overview.">
    <RouterLink to="/" class="inline-flex min-h-11 items-center rounded-md bg-primary px-4 text-sm font-semibold text-white">Back to overview</RouterLink>
  </EmptyState>
</template>
