import { afterEach, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { useToastStore } from "../toast";
afterEach(() => vi.useRealTimers());
it("keeps errors and recovery actions until dismissed while successes expire", () => {
  vi.useFakeTimers(); setActivePinia(createPinia()); const store = useToastStore();
  const error = store.error("Could not save the draft");
  store.success("Saved"); store.push("info", "Review the race", undefined, { label: "Open race", to: "/events" });
  vi.advanceTimersByTime(60_000);
  expect(store.toasts.map(toast => toast.message)).toEqual(["Could not save the draft", "Review the race"]);
  store.dismiss(error); expect(store.toasts).toHaveLength(1);
});
