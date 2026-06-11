<script setup lang="ts">
// Port of pw-btn: primary / ghost / dark / danger / success variants
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
  primary: "bg-accent/15 text-accent border-accent/40 hover:bg-accent/25",
  ghost: "bg-transparent text-muted border-line hover:border-line-hi hover:text-text",
  dark: "bg-surface-2 text-muted border-line hover:text-text hover:border-line-hi",
  danger: "bg-danger-glow text-danger border-danger/40 hover:bg-danger/20",
  success: "bg-ok-glow text-ok border-ok/40 hover:bg-ok/20",
};
</script>

<template>
  <button
    :type="type"
    :disabled="disabled"
    class="inline-flex items-center justify-center gap-2 rounded-md border font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-45"
    :class="[variants[variant], size === 'sm' ? 'px-2.5 py-1 text-xs' : 'px-3.5 py-1.5 text-sm']"
  >
    <slot />
  </button>
</template>
