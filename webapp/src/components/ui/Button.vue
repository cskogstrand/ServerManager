<script setup lang="ts">
// Semantic colors keep actions readable in both Paddock themes.
withDefaults(
  defineProps<{
    variant?: "primary" | "ghost" | "dark" | "danger" | "success";
    size?: "md" | "sm";
    disabled?: boolean;
    type?: "button" | "submit";
  }>(),
  { variant: "primary", size: "md", disabled: false, type: "button" },
);

const variants: Record<string, string> = {
  primary: "bg-primary text-on-primary border-primary hover:brightness-110",
  ghost: "bg-transparent text-muted border-line hover:border-line-hi hover:bg-surface-2/70 hover:text-text",
  dark: "bg-surface-2 text-muted border-line hover:bg-surface-3 hover:text-text hover:border-line-hi",
  danger: "bg-danger-glow text-danger border-danger/45 hover:bg-danger/20 hover:border-danger/70",
  success: "bg-ok-glow text-ok border-ok/45 hover:bg-ok/20 hover:border-ok/70",
};
</script>

<template>
  <button
    :type="type"
    :disabled="disabled"
    class="inline-flex cursor-pointer items-center justify-center gap-2 rounded-md border font-semibold transition-colors duration-200 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent disabled:cursor-not-allowed disabled:opacity-45"
    :class="[variants[variant], size === 'sm' ? 'min-h-11 px-3 text-sm' : 'min-h-11 px-4 text-sm']"
  >
    <slot />
  </button>
</template>
