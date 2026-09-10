import type { ComputedRef, InjectionKey } from "vue";

export const formFieldKey: InjectionKey<{
  id: ComputedRef<string>;
  labelId: string;
  describedBy: ComputedRef<string | undefined>;
  invalid: ComputedRef<boolean>;
}> = Symbol("form-field");
