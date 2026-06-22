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
// feedback is consistent and non-blocking (errors persist longer than
// successes; SSE-driven events can push toasts from anywhere).
export const useToastStore = defineStore("toast", {
  state: () => ({
    toasts: [] as Toast[],
  }),

  actions: {
    push(tone: ToastTone, message: string, ttl?: number, action?: ToastAction) {
      const id = nextId++;
      // Actionable toasts linger a little longer so the button is clickable.
      const duration = ttl ?? (tone === "error" ? 8000 : action ? 7000 : 4000);
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
