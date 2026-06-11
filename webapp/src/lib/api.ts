// Typed API client. New endpoints return {"error": {"code","message"}} on
// failure; a few legacy ones still return {"success": false, "message"}.
// Both are normalized into ApiError.

export class ApiError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export function csrfToken(): string {
  const m = document.cookie.match(/(?:^|; )csrf_token=([^;]+)/);
  return m ? m[1] : "";
}

async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};
  const mutating = method !== "GET" && method !== "HEAD";
  if (mutating) {
    headers["X-CSRF-Token"] = csrfToken();
  }
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }

  const res = await fetch(url, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (res.status === 401 || (res.redirected && res.url.includes("/login"))) {
    throw new ApiError(401, "unauthorized", "Not logged in");
  }

  const text = await res.text();
  let json: any = null;
  if (text) {
    try {
      json = JSON.parse(text);
    } catch {
      if (!res.ok) throw new ApiError(res.status, "http_error", res.statusText);
      return text as unknown as T;
    }
  }

  if (!res.ok || json?.error || json?.success === false) {
    const code = json?.error?.code ?? "http_error";
    const message = json?.error?.message ?? json?.message ?? res.statusText;
    throw new ApiError(res.status, code, message);
  }

  return json as T;
}

export const api = {
  get: <T>(url: string) => request<T>("GET", url),
  post: <T>(url: string, body?: unknown) => request<T>("POST", url, body),
  put: <T>(url: string, body?: unknown) => request<T>("PUT", url, body),
  delete: <T>(url: string) => request<T>("DELETE", url),
};
