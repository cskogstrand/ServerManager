import { computed, type Ref, type WritableComputedRef } from "vue";

// Bridges a nullable 0/1 int field (Go *int) to a Toggle's boolean model.
export function intToggle<T extends Record<string, any>>(
  form: Ref<T | null>,
  key: keyof T,
): WritableComputedRef<boolean> {
  return computed({
    get: () => (form.value?.[key] ?? 0) === 1,
    set: (v: boolean) => {
      if (form.value) (form.value as any)[key] = v ? 1 : 0;
    },
  });
}
