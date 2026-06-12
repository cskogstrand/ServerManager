import { onBeforeUnmount } from "vue";
import { onBeforeRouteLeave } from "vue-router";
import { useConfirmStore } from "@/stores/confirm";

// Warns before navigating away (or closing the tab) while a form has unsaved
// changes. Pass a getter that reports whether the form is currently dirty.
export function useUnsavedGuard(isDirty: () => boolean) {
  const confirm = useConfirmStore();

  const beforeUnload = (e: BeforeUnloadEvent) => {
    if (isDirty()) {
      e.preventDefault();
      e.returnValue = "";
    }
  };
  window.addEventListener("beforeunload", beforeUnload);
  onBeforeUnmount(() => window.removeEventListener("beforeunload", beforeUnload));

  onBeforeRouteLeave(async () => {
    if (!isDirty()) return true;
    return await confirm.ask({
      title: "Discard changes?",
      message: "You have unsaved changes on this page.",
      detail: "Leave without saving them?",
      confirmLabel: "Discard changes",
      cancelLabel: "Stay",
      tone: "danger",
    });
  });
}
