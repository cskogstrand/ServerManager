// Guest drivers: the roster of real people who may share one Assetto Corsa
// account (GUID), each with an avatar and its own profile page, plus the two
// ways to attribute results to them — reassigning a past leaderboard row, and
// tagging a live, connected car. Backed by /api/guest-drivers,
// /api/scores/:id/assign and /api/server/assign-driver.
import { api, ApiError } from "@/lib/api";
import type { GuestDriverDetail } from "@/types/driverStats";

export interface GuestDriver {
  id: number;
  name: string;
  notes?: string;
  avatar_url?: string | null;
  created_at: number;
}

export async function listGuestDrivers(): Promise<GuestDriver[]> {
  return (await api.get<{ items: GuestDriver[] }>("/api/guest-drivers")).items ?? [];
}

// Full profile (KPIs, favourites, history, highlight clips) for one guest, or
// null when the id is unknown.
export async function getGuestDriver(id: number): Promise<GuestDriverDetail | null> {
  try {
    return await api.get<GuestDriverDetail>(`/api/guest-drivers/${id}`);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return null;
    throw e;
  }
}

export async function createGuestDriver(name: string, notes: string): Promise<number> {
  return (await api.post<{ id: number }>("/api/guest-drivers", { name, notes })).id;
}

export async function updateGuestDriver(id: number, name: string, notes: string): Promise<void> {
  await api.put(`/api/guest-drivers/${id}`, { name, notes });
}

export async function deleteGuestDriver(id: number): Promise<void> {
  await api.delete(`/api/guest-drivers/${id}`);
}

// Reassign a single leaderboard row to a guest driver, or pass null to clear the
// attribution (revert to the GUID's own name). `scoreId` is the leaderboard id
// ("d<runId>" for a drift run, "l<sessionId>" for a timed lap).
export async function assignScore(scoreId: string, guestDriverId: number | null): Promise<void> {
  await api.post(`/api/scores/${encodeURIComponent(scoreId)}/assign`, { guest_driver_id: guestDriverId });
}

// Attribute an entire session (connection) on the driver detail page to a guest
// driver, or pass null to clear it back to the GUID's own name. `sessionId` is
// the connection id (DriverSession.id). Cascades to every row in the stint.
export async function assignSession(guid: string, sessionId: string, guestDriverId: number | null): Promise<void> {
  await api.post(`/api/drivers/${encodeURIComponent(guid)}/sessions/${encodeURIComponent(sessionId)}/assign`, {
    guest_driver_id: guestDriverId,
  });
}

// Tag a currently-connected car on a running instance with a guest driver (or
// null to clear) so its subsequent runs/sessions are recorded under that person.
export async function assignLiveDriver(instanceId: number, carId: number, guestDriverId: number | null): Promise<void> {
  await api.post(`/api/server/assign-driver?instance=${instanceId}`, {
    car_id: carId,
    guest_driver_id: guestDriverId,
  });
}
