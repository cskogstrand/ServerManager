import { onUnmounted, ref, watch, type Ref } from "vue";

// Animate a number toward `target` with an ease-out cubic. Used for the hero
// KPIs on the driver detail page — a single high-impact reveal beats scattered
// micro-motion. Honors prefers-reduced-motion by snapping straight to the value.
export function useCountUp(target: Ref<number>, durationMs = 850): Ref<number> {
  const out = ref(0);
  const reduce =
    typeof window !== "undefined" && typeof window.matchMedia === "function"
      ? window.matchMedia("(prefers-reduced-motion: reduce)").matches
      : false;

  let raf = 0;
  let from = 0;
  let startTs = 0;

  function frame(ts: number) {
    if (!startTs) startTs = ts;
    const p = Math.min(1, (ts - startTs) / durationMs);
    const eased = 1 - Math.pow(1 - p, 3);
    out.value = Math.round(from + (target.value - from) * eased);
    if (p < 1) raf = requestAnimationFrame(frame);
  }

  watch(
    target,
    (v) => {
      cancelAnimationFrame(raf);
      if (reduce) {
        out.value = v;
        return;
      }
      from = out.value;
      startTs = 0;
      raf = requestAnimationFrame(frame);
    },
    { immediate: true },
  );

  onUnmounted(() => cancelAnimationFrame(raf));
  return out;
}
