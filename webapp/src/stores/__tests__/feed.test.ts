import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";

// Stub the content store dependency pulled in transitively (driversApi).
vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), post: vi.fn(), delete: vi.fn() },
  ApiError: class ApiError extends Error {},
}));

import { useFeedStore, feedItemFromEvent } from "@/stores/feed";
import { useToastStore } from "@/stores/toast";
import type { ServerEvent } from "@/lib/sse";

function driftEvent(guid: string, score: number): ServerEvent {
  return { type: "drift_run", instance_id: 1, ts: 0, data: { guid, name: "Kaz", score } };
}

describe("feed store", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.useRealTimers();
  });

  it("adds every event to the feed but throttles repeat toasts per driver", () => {
    const feed = useFeedStore();
    const toast = useToastStore();

    feed.ingest(driftEvent("g1", 1000));
    feed.ingest(driftEvent("g1", 2000)); // same driver, within window
    feed.ingest(driftEvent("g2", 500)); // different driver — own window

    // Feed keeps all three; toasts collapse the g1 burst to one.
    expect(feed.items.length).toBe(3);
    expect(toast.toasts.length).toBe(2);
    expect(feed.items[0].guid).toBe("g2");
  });

  it("does not toast for laps (feed-only) but still records them", () => {
    const feed = useFeedStore();
    const toast = useToastStore();
    feed.ingest({ type: "lap", instance_id: 1, ts: 0, data: { guid: "g1", name: "Kaz", lap: 3, laptime_ms: 91234, cuts: 0 } });
    expect(feed.items.length).toBe(1);
    expect(toast.toasts.length).toBe(0);
  });
});

it("opens source media at its camera instead of treating a source ID as a driver", () => {
  expect(feedItemFromEvent({type:"media", instance_id:0, ts:0, data:{guid:"driver-7",source_id:"source:7",file:"clip.mp4",kind:"clip"}})?.link).toBe("/garage/cameras/source%3A7?media=clip.mp4");
});
