import axios, { type AxiosInstance, type AxiosRequestConfig } from "axios";

/** Normalized API failure; status 0 denotes a transport or response-format error. */
export class ApiError extends Error {
  constructor(public status: number, public code: string) {
    super(code);
  }
}

const baseURL = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081").replace(/\/$/, "");

/** Resolves an API-provided root-relative upload path against the API origin. */
export function fileURL(path: string): string {
  return `${baseURL}${path}`;
}

export type APIRequestOptions = Pick<AxiosRequestConfig, "method" | "headers" | "data" | "signal">;

/** Creates a cookie-enabled JSON client; Bearer injection is configured by the session module. */
export function createAPIClient(): AxiosInstance {
  return axios.create({
    baseURL: `${baseURL}/api/`,
    withCredentials: true,
    timeout: 15000,
    responseType: "json",
    transitional: { silentJSONParsing: false },
  });
}

const publicClient = createAPIClient();

/** Returns the response body (undefined for 204) and normalizes Axios failures to ApiError. */
export async function apiRequest<T>(path: string, options: APIRequestOptions = {}, client = publicClient): Promise<T> {
  try {
    const response = await client.request<T>({ ...options, url: path });
    return response.status === 204 ? undefined as T : response.data;
  } catch (error) {
    if (!axios.isAxiosError<{ error?: { code?: string } }>(error)) throw error;
    if (error.response && (error.response.status < 200 || error.response.status >= 300)) {
      const code = error.response.data?.error?.code;
      throw new ApiError(error.response.status, typeof code === "string" && code ? code : "request_failed");
    }
    if (error.code === "ERR_BAD_RESPONSE") throw new ApiError(0, "invalid_response");
    throw new ApiError(0, "network_error");
  }
}
