import { defineStore } from "pinia";
import type { ServerEvent } from "@/lib/sse";
import { useToastStore, type ToastTone } from "@/stores/toast";
import { fmtScore, shortGuid } from "@/lib/driversApi";
import { lapTime } from "@/lib/raceTelemetry";

// One global "recent events" feed, fed by the SSE stream. Drives the dashboard
// live-feed strip and the actionable toast notifications. A capped ring — this
// is a recent-activity view, not a log.
export type FeedTone = "info" | "ok" | "warn" | "accent" | "danger" | "dim";

export interface FeedItem {
  id: number;
  type: string;
  icon: string;
  tone: FeedTone;
  text: string;
  guid?: string;
  link?: string;
  ts: number;
}

const CAP = 40;
let seq = 1;

// Per (type+guid) toast throttle: a drift server can finish runs every few
// seconds; the feed keeps every one, but we don't fire a toast for each.
// ponytail: fixed 6s window, make it configurable only if someone asks.
const lastToastAt: Record<string, number> = {};
const TOAST_WINDOW = 6000;

interface Describe {
  icon: string;
  tone: FeedTone;
  text: string;
  guid?: string;
  link?: string;
  notify?: boolean;
  action?: string;
  toastTone?: ToastTone;
}

function describe(e: ServerEvent): Describe | null {
  const d = e.data ?? {};
  const guid: string | undefined = d.guid || undefined;
  const link = guid ? `/drivers/${encodeURIComponent(guid)}` : undefined;
  const who = d.name || (guid ? shortGuid(guid) : "Driver");

  switch (e.type) {
    case "session_start":
      return { icon: "users", tone: "info", text: `${who} joined`, guid, link };
    case "session_end": {
      const detail =
        d.drift_best > 0
          ? `best drift ${fmtScore(d.drift_best)}`
          : d.best_lap_ms > 0
            ? `best ${lapTime(d.best_lap_ms)}`
            : `${d.laps} lap${d.laps === 1 ? "" : "s"}`;
      const place = d.finish_pos > 0 ? ` · P${d.finish_pos}${d.entrants ? `/${d.entrants}` : ""}` : "";
      return {
        icon: "flag", tone: "accent", text: `${who} finished — ${detail}${place}`,
        guid, link, notify: true, action: "View driver", toastTone: "info",
      };
    }
    case "lap":
      return {
        icon: "gauge", tone: "info",
        text: `${who} · lap ${d.lap} · ${lapTime(d.laptime_ms)}${d.cuts ? " (cut)" : ""}`,
        guid, link,
      };
    case "drift_run":
      return {
        icon: "trophy", tone: "warn", text: `${who} scored ${fmtScore(d.score)} pts`,
        guid, link, notify: true, action: "View", toastTone: "success",
      };
    case "media": {
      const clip = d.kind === "clip";
      return {
        icon: clip ? "film" : "camera", tone: "accent",
        text: `${clip ? "Clip" : "Picture"} saved${d.caption ? ` — ${d.caption}` : ""}`,
        guid, link, notify: true, action: "Preview now", toastTone: "success",
      };
    }
    case "recording":
      return {
        icon: "record", tone: "danger", text: `${who} · recording started`,
        guid, link, notify: true, action: "Open", toastTone: "info",
      };
    default:
      return null;
  }
}

export const useFeedStore = defineStore("feed", {
  state: () => ({
    items: [] as FeedItem[],
    // Bumped on every push so views (e.g. driver detail) can watch for activity
    // without diffing the whole list.
    lastId: 0,
  }),

  actions: {
    // add pushes a pre-built item (used for non-driver events like server start).
    add(item: Omit<FeedItem, "id" | "ts"> & { ts?: number }) {
      const full: FeedItem = { id: seq++, ts: item.ts ?? Date.now(), ...item };
      this.items.unshift(full);
      if (this.items.length > CAP) this.items.length = CAP;
      this.lastId = full.id;
      return full;
    },

    // ingest maps a raw domain SSE event to a feed item and, when notable, an
    // actionable toast (throttled per type+guid).
    ingest(e: ServerEvent) {
      const v = describe(e);
      if (!v) return;
      this.add({
        type: e.type, icon: v.icon, tone: v.tone, text: v.text,
        guid: v.guid, link: v.link, ts: e.ts ? e.ts * 1000 : Date.now(),
      });
      if (!v.notify) return;
      const key = `${e.type}:${v.guid ?? ""}`;
      const now = Date.now();
      if (now - (lastToastAt[key] ?? 0) < TOAST_WINDOW) return;
      lastToastAt[key] = now;
      useToastStore().push(
        v.toastTone ?? "info",
        v.text,
        undefined,
        v.link ? { label: v.action ?? "Open", to: v.link } : undefined,
      );
    },
  },
});
