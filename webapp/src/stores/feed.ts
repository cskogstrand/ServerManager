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
    case "app":
      // Server Manager app start/stop (persisted; replays after a restart).
      return d.running
        ? { icon: "activity", tone: "ok", text: "Server Manager started" }
        : { icon: "activity", tone: "dim", text: "Server Manager stopped" };
    case "server":
      // Server start/stop. Persisted so the line survives a refresh; the toast
      // stays in the server store (notify:false here to avoid a double toast).
      return d.running
        ? { icon: "power", tone: "ok", text: `${d.name ?? "Server"} started`, link: `/server/${e.instance_id}` }
        : { icon: "power", tone: "dim", text: `${d.name ?? "Server"} stopped` };
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
      return d.recording === false
        ? {
            icon: "film", tone: "dim", text: `${who} · recording stopped — saving clip`,
            guid, link, notify: true, action: "Open", toastTone: "info",
          }
        : {
            icon: "record", tone: "danger", text: `${who} · recording started`,
            guid, link, notify: true, action: "Open", toastTone: "info",
          };
    default:
      return null;
  }
}

// feedItemFromEvent maps a raw domain event to a display item (no side effects).
// Shared by the live SSE ingest and the persisted-feed history fetch so both
// render identically. Returns null for events that aren't feed-worthy.
export function feedItemFromEvent(e: ServerEvent): FeedItem | null {
  const v = describe(e);
  if (!v) return null;
  return {
    id: seq++, type: e.type, icon: v.icon, tone: v.tone, text: v.text,
    guid: v.guid, link: v.link, ts: e.ts ? e.ts * 1000 : Date.now(),
  };
}

export const useFeedStore = defineStore("feed", {
  state: () => ({
    items: [] as FeedItem[],
    // Bumped on every push so views (e.g. driver detail) can watch for activity
    // without diffing the whole list.
    lastId: 0,
  }),

  actions: {
    // seed fills the live ring from persisted history (newest-first) so a fresh
    // page load isn't empty. Only seeds when the ring is empty — never clobbers
    // events the live SSE stream has already delivered.
    seed(events: ServerEvent[]) {
      if (this.items.length) return;
      const items = events.map(feedItemFromEvent).filter((i): i is FeedItem => i !== null);
      this.items = items.slice(0, CAP);
      this.lastId = this.items[0]?.id ?? this.lastId;
    },

    // ingest maps a raw domain SSE event to a feed item and, when notable, an
    // actionable toast (throttled per type+guid).
    ingest(e: ServerEvent) {
      const item = feedItemFromEvent(e);
      if (!item) return;
      this.items.unshift(item);
      if (this.items.length > CAP) this.items.length = CAP;
      this.lastId = item.id;

      const v = describe(e);
      if (!v?.notify) return;
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
