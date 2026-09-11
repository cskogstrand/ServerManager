// Driver history always comes from the authenticated backend.
import { api, ApiError } from "@/lib/api";
import { lapTime } from "@/lib/raceTelemetry";
import type {
  DriverDetail,
  DriverResult,
  DriverSummary,
  ScoreEntry,
  SessionSearchResult,
} from "@/types/driverStats";

// ---- formatting helpers -----------------------------------------------------

export function fmtScore(n: number): string {
  return Math.round(n).toLocaleString("en-US");
}

// Steam64 guids are long; show a stable short tail for the list/detail chips.
export function shortGuid(guid: string): string {
  if (guid.length <= 8) return guid;
  return `…${guid.slice(-6)}`;
}

export function timeAgo(ms: number): string {
  const diff = Date.now() - ms;
  if (diff < 0) return "just now";
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;
  return `${Math.floor(months / 12)}y ago`;
}

export function fmtDate(ms: number): string {
  return new Date(ms).toLocaleDateString(undefined, { day: "2-digit", month: "short", year: "numeric" });
}

export const sessionKindLabel: Record<string, string> = {
  race: "Race",
  qualify: "Qualify",
  practice: "Practice",
  drift: "Drift",
};

export interface ResultView {
  primary: string;
  secondary: string;
  unit: string;
  tone: "accent" | "warn" | "text";
}

// Collapse a session into a single headline metric. Drift runs read as a score;
// timed sessions read as the best lap with finishing position. A win glows amber.
export function describeResult(r: DriverResult | null): ResultView {
  if (!r) return { primary: "—", secondary: "No sessions yet", unit: "", tone: "text" };
  if (r.kind === "drift") {
    return { primary: fmtScore(r.drift_score ?? 0), secondary: r.track.name, unit: "PTS", tone: "accent" };
  }
  const pos = r.position ?? 0;
  const posLabel = pos ? `P${pos}${r.entrants ? `/${r.entrants}` : ""}` : "—";
  return {
    primary: lapTime(r.best_lap_ms ?? 0),
    secondary: `${posLabel} · ${sessionKindLabel[r.kind]} · ${r.track.name}`,
    unit: "BEST LAP",
    tone: pos === 1 ? "warn" : "text",
  };
}

// ---- public API -------------------------------------------------------------

export async function listDrivers(): Promise<DriverSummary[]> {
 return (await api.get<{drivers:DriverSummary[]}>("/api/drivers")).drivers??[];
}
export async function listScores(): Promise<ScoreEntry[]> {
 return (await api.get<{scores:ScoreEntry[]}>("/api/scores")).scores??[];
}
export async function getDriver(guid:string):Promise<DriverDetail|null>{
 try{return await api.get<DriverDetail>(`/api/drivers/${encodeURIComponent(guid)}`)}catch(e){if(e instanceof ApiError&&e.status===404)return null;throw e}
}

// Add a tag to a session (connection). Returns the session's full tag set.
export async function addSessionTag(guid: string, sessionId: string, tag: string): Promise<string[]> {
  const res = await api.post<{ tags: string[] }>(
    `/api/drivers/${encodeURIComponent(guid)}/sessions/${encodeURIComponent(sessionId)}/tags`,
    { tag },
  );
  return res.tags ?? [];
}

// Remove a tag from a session. Returns the remaining tags.
export async function removeSessionTag(guid: string, sessionId: string, tag: string): Promise<string[]> {
  const res = await api.delete<{ tags: string[] }>(
    `/api/drivers/${encodeURIComponent(guid)}/sessions/${encodeURIComponent(sessionId)}/tags?tag=${encodeURIComponent(tag)}`,
  );
  return res.tags ?? [];
}

// Delete an entire session (connection) and everything tied to it: drift
// scores, lap times and captured media (files included). No undo.
export async function deleteSession(guid: string, sessionId: string): Promise<void> {
  await api.delete(`/api/drivers/${encodeURIComponent(guid)}/sessions/${encodeURIComponent(sessionId)}`);
}

// Global historical connection search retains the legacy query contract.
export async function searchSessions(opts:{tag?:string;q?:string}={}):Promise<SessionSearchResult[]>{
 const params=new URLSearchParams();if(opts.tag)params.set('tag',opts.tag);if(opts.q)params.set('q',opts.q);
 return (await api.get<{sessions:SessionSearchResult[]}>(`/api/driver-sessions?${params}`)).sessions??[];
}
