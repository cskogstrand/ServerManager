import { defineStore } from "pinia";

export interface ConfirmOptions {
  title?: string;
  message: string;
  // Spell out consequences, e.g. "Removes 3 queue entries."
  detail?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  tone?: "danger" | "default";
}

interface ConfirmState extends ConfirmOptions {
  open: boolean;
  resolve: ((ok: boolean) => void) | null;
}

// Promise-based confirm dialog. Replaces window.confirm so consequences can be
// spelled out and styling stays consistent. Usage:
//   if (!(await confirm.ask({ message, detail, tone: "danger" }))) return;
export const useConfirmStore = defineStore("confirm", {
  state: (): ConfirmState => ({
    open: false,
    message: "",
    resolve: null,
  }),

  actions: {
    ask(opts: ConfirmOptions): Promise<boolean> {
      this.open = true;
      this.title = opts.title ?? "Are you sure?";
      this.message = opts.message;
      this.detail = opts.detail;
      this.confirmLabel = opts.confirmLabel ?? "Confirm";
      this.cancelLabel = opts.cancelLabel ?? "Cancel";
      this.tone = opts.tone ?? "default";
      return new Promise<boolean>((resolve) => {
        this.resolve = resolve;
      });
    },
    answer(ok: boolean) {
      this.open = false;
      this.resolve?.(ok);
      this.resolve = null;
    },
  },
});
