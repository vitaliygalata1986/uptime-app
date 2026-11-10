export class ApiError extends Error {
  constructor(public status: number, public code: string) {
    super(code);
  }
}

const baseURL = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081").replace(/\/$/, "");

export function fileURL(path: string): string {
  return `${baseURL}${path}`;
}

export async function apiRequest<T>(path: string, options: RequestInit = {}): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${baseURL}/api/${path}`, {
      ...options,
      credentials: "include",
      cache: "no-store",
      signal: AbortSignal.timeout(15000),
    });
  } catch {
    throw new ApiError(0, "network_error");
  }
  if (!response.ok) {
    const body = await response.json().catch(() => null);
    throw new ApiError(response.status, body?.error?.code || "request_failed");
  }
  if (response.status === 204) return undefined as T;
  try {
    return await response.json() as T;
  } catch {
    throw new ApiError(0, "invalid_response");
  }
}
