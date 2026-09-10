export function queueState(row: { finished: number | boolean; started_at?: number | null }, running: boolean): "done" | "active" | "pending" {
  if (row.finished) return "done";
  return running && !!row.started_at ? "active" : "pending";
}
