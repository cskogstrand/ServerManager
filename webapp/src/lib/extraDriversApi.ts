// Extra drivers: the roster of real people who may share one Assetto Corsa
// account (GUID), plus the two ways to attribute results to them — reassigning a
// past leaderboard row, and tagging a live, connected car. Backed by /api/extra-
// drivers, /api/scores/:id/assign and /api/server/assign-driver.
import { api } from "@/lib/api";

export interface ExtraDriver {
  id: number;
  name: string;
  notes?: string;
  created_at: number;
}

export async function listExtraDrivers(): Promise<ExtraDriver[]> {
  return (await api.get<{ items: ExtraDriver[] }>("/api/extra-drivers")).items ?? [];
}

export async function createExtraDriver(name: string, notes: string): Promise<number> {
  return (await api.post<{ id: number }>("/api/extra-drivers", { name, notes })).id;
}

export async function updateExtraDriver(id: number, name: string, notes: string): Promise<void> {
  await api.put(`/api/extra-drivers/${id}`, { name, notes });
}

export async function deleteExtraDriver(id: number): Promise<void> {
  await api.delete(`/api/extra-drivers/${id}`);
}

// Reassign a single leaderboard row to an extra driver, or pass null to clear the
// attribution (revert to the GUID's own name). `scoreId` is the leaderboard id
// ("d<runId>" for a drift run, "l<sessionId>" for a timed lap).
export async function assignScore(scoreId: string, extraDriverId: number | null): Promise<void> {
  await api.post(`/api/scores/${encodeURIComponent(scoreId)}/assign`, { extra_driver_id: extraDriverId });
}

// Tag a currently-connected car on a running instance with an extra driver (or
// null to clear) so its subsequent runs/sessions are recorded under that person.
export async function assignLiveDriver(instanceId: number, carId: number, extraDriverId: number | null): Promise<void> {
  await api.post(`/api/server/assign-driver?instance=${instanceId}`, {
    car_id: carId,
    extra_driver_id: extraDriverId,
  });
}
