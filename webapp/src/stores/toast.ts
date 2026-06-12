import { defineStore } from "pinia";

export type ToastTone = "success" | "error" | "info";

export interface Toast {
  id: number;
  tone: ToastTone;
  message: string;
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
    push(tone: ToastTone, message: string, ttl?: number) {
      const id = nextId++;
      const duration = ttl ?? (tone === "error" ? 8000 : 4000);
      const timer = duration > 0 ? setTimeout(() => this.dismiss(id), duration) : null;
      this.toasts.push({ id, tone, message, timer });
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
