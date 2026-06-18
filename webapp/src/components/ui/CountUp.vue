<script setup lang="ts">
// Thin presentational wrapper around useCountUp for integer metrics; renders
// the running value with locale thousands separators.
import { toRef } from "vue";
import { useCountUp } from "@/lib/useCountUp";

const props = defineProps<{ value: number }>();
const display = useCountUp(toRef(props, "value"));
</script>

<template>
  <span>{{ display.toLocaleString() }}</span>
</template>
