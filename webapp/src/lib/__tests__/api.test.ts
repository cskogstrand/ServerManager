import { afterEach, describe, expect, it, vi } from "vitest";
import { api, ApiError, csrfToken } from "../api";

function mockFetch(status: number, body: unknown) {
  const fn = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    statusText: "status",
    redirected: false,
    url: "http://localhost/api/test",
    text: () => Promise.resolve(JSON.stringify(body)),
  });
  vi.stubGlobal("fetch", fn);
  return fn;
}

afterEach(() => {
  vi.unstubAllGlobals();
  document.cookie = "csrf_token=; expires=Thu, 01 Jan 1970 00:00:00 GMT";
});

describe("api client", () => {
  it("reads the csrf cookie", () => {
    document.cookie = "csrf_token=abc123";
    expect(csrfToken()).toBe("abc123");
  });

  it("sends X-CSRF-Token on mutating requests only", async () => {
    document.cookie = "csrf_token=tok";
    const fn = mockFetch(200, { id: 1 });

    await api.get("/api/difficulties");
    expect(fn.mock.calls[0][1].headers["X-CSRF-Token"]).toBeUndefined();

    await api.post("/api/difficulties", { name: "Easy" });
    expect(fn.mock.calls[1][1].headers["X-CSRF-Token"]).toBe("tok");
    expect(fn.mock.calls[1][1].headers["Content-Type"]).toBe("application/json");
  });

  it("throws ApiError from the error envelope", async () => {
    mockFetch(409, { error: { code: "in_use", message: "still referenced" } });
    const err = (await api.delete("/api/difficulty/1").catch((e) => e)) as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("in_use");
    expect(err.status).toBe(409);
    expect(err.message).toBe("still referenced");
  });

  it("normalizes legacy {success:false} responses", async () => {
    mockFetch(200, { success: false, message: "legacy failure" });
    const err = (await api.post("/api/server/current-event", {}).catch((e) => e)) as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).toBe("legacy failure");
  });
});
