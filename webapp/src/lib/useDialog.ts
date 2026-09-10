import { computed, onBeforeUnmount, ref, shallowRef, watch, type Ref } from "vue";

// Keep feedback inside the active modal so it stays visible and interactive.
const dialogStack = shallowRef<HTMLDialogElement[]>([]);
export const activeDialog = computed(() => dialogStack.value.at(-1) ?? null);
const forget = (element: HTMLDialogElement | null) => { dialogStack.value = dialogStack.value.filter(item => item !== element); };

// Native modal dialogs provide focus containment, background inertness and a
// top layer that also works when a picker or confirmation opens over an editor.
export function useDialog(open: Ref<boolean>) {
  const dialog = ref<HTMLDialogElement | null>(null);
  let trigger: HTMLElement | null = null;
  function wrapFocus(event: KeyboardEvent) {
    const element = dialog.value;
    if (event.key !== "Tab" || !element || activeDialog.value !== element) return;
    const controls = Array.from(element.querySelectorAll<HTMLElement>("button, a[href], input, select, textarea, summary, [tabindex]"))
      .filter(item => item.tabIndex >= 0 && !item.matches(":disabled, [inert], [hidden]") && item.getClientRects().length > 0);
    const first = controls[0]; const last = controls.at(-1);
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
  }
  watch([open, dialog], ([isOpen, element]) => {
    if (!element) return;
    if (isOpen && !element.open) {
      trigger = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      element.addEventListener("keydown", wrapFocus);
      element.showModal();
      dialogStack.value = [...dialogStack.value, element];
    } else if (!isOpen && element.open) {
      element.removeEventListener("keydown", wrapFocus);
      forget(element);
      element.close();
      if (trigger?.isConnected) trigger.focus({ preventScroll: true });
      trigger = null;
    }
  }, { flush: "post" });
  onBeforeUnmount(() => {
    dialog.value?.removeEventListener("keydown", wrapFocus);
    forget(dialog.value);
    if (dialog.value?.open) dialog.value.close();
    if (trigger?.isConnected) trigger.focus({ preventScroll: true });
  });
  return dialog;
}
