export interface User {
  id: number;
  email: string;
  created_at: string;
}

export interface AuthResult {
  user: User;
  access_token: string;
  token_type: "Bearer";
  expires_in: number;
}

export class ApiError extends Error {
  constructor(public status: number, public code: string) {
    super(code);
  }
}

const baseURL = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081").replace(/\/$/, "");

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${baseURL}/api/auth/${path}`, {
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

export function post<T>(path: string, body: object = {}) {
  return request<T>(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function errorMessage(error: unknown): string {
  if (!(error instanceof ApiError)) return "Не удалось выполнить запрос. Попробуйте ещё раз.";
  if (error.status === 401) return "Неверный email или пароль.";
  if (error.status === 409) return "Этот email уже зарегистрирован. Войдите в аккаунт.";
  if (error.status === 429) return "Слишком много попыток. Подождите минуту и попробуйте снова.";
  if (error.status === 400) return "Проверьте email и пароль: пароль должен содержать от 8 до 128 символов.";
  if (error.code === "unsupported_browser") return "Откройте приложение в современном браузере через localhost или HTTPS.";
  return "Сервис временно недоступен. Проверьте подключение и попробуйте снова.";
}
