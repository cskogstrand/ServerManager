import { onBeforeUnmount } from "vue";
import { onBeforeRouteLeave } from "vue-router";
import { useConfirmStore } from "@/stores/confirm";

// Warns before navigating away (or closing the tab) while a form has unsaved
// changes. Pass a getter that reports whether the form is currently dirty.
export function useUnsavedGuard(isDirty: () => boolean, routeGuard = true) {
  const confirm = useConfirmStore();

  const beforeUnload = (e: BeforeUnloadEvent) => {
    if (isDirty()) {
      e.preventDefault();
      e.returnValue = "";
    }
  };
  window.addEventListener("beforeunload", beforeUnload);
  onBeforeUnmount(() => window.removeEventListener("beforeunload", beforeUnload));

  let pending: Promise<boolean> | null = null;
  const canLeave = () => {
    if (!isDirty()) return Promise.resolve(true);
    if (pending) return pending;
    pending = confirm.ask({
      title: "Discard unsaved changes?",
      message: "Your changes have not been saved.",
      detail: "Keep editing to preserve them, or discard these changes.",
      confirmLabel: "Discard changes",
      cancelLabel: "Keep editing",
      tone: "danger",
    }).finally(() => { pending = null; });
    return pending;
  };
  if (routeGuard) onBeforeRouteLeave(canLeave);
  // The same guard must run for X, backdrop, Escape and Cancel, not only routes.
  return async (close: () => void | Promise<void>) => { if (await canLeave()) await close(); };
}
