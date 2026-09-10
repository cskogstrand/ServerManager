import { defineStore } from "pinia";

export type ToastTone = "success" | "error" | "info";

// An optional actionable button on a toast — a router target ("Go to race",
// "Preview now"). Clicking it navigates and dismisses the toast.
export interface ToastAction {
  label: string;
  to: string;
}

export interface Toast {
  id: number;
  tone: ToastTone;
  message: string;
  action?: ToastAction;
  // Auto-dismiss timer handle, kept so we can clear it on manual dismiss.
  timer: ReturnType<typeof setTimeout> | null;
}

let nextId = 1;

// Global toast stack. Replaces the per-page inline notice/error rows so
// feedback is consistent; errors stay visible until dismissed.
export const useToastStore = defineStore("toast", {
  state: () => ({
    toasts: [] as Toast[],
  }),

  actions: {
    push(tone: ToastTone, message: string, ttl?: number, action?: ToastAction) {
      const id = nextId++;
      // Errors and actions remain available until the user dismisses them.
      const duration = ttl ?? (tone === "error" || action ? 0 : 4000);
      const timer = duration > 0 ? setTimeout(() => this.dismiss(id), duration) : null;
      this.toasts.push({ id, tone, message, action, timer });
      return id;
    },
    success(message: string, ttl?: number) {
      return this.push("success", message, ttl);
    },
    error(message: string, ttl?: number) {
      return this.push("error", message, ttl);
    },
    info(message: string, ttl?: number) {
      return this.push("info", message, ttl);
    },
    dismiss(id: number) {
      const i = this.toasts.findIndex((t) => t.id === id);
      if (i === -1) return;
      const t = this.toasts[i];
      if (t.timer) clearTimeout(t.timer);
      this.toasts.splice(i, 1);
    },
  },
});
