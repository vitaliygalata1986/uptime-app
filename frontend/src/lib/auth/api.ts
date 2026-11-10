import { apiRequest, ApiError } from "../api";

export { ApiError, fileURL as avatarURL } from "../api";

export interface User {
  id: number;
  email: string;
  name: string;
  avatar_url: string;
  created_at: string;
}

export interface AuthResult {
  user: User;
  access_token: string;
  token_type: "Bearer";
  expires_in: number;
}

export function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  return apiRequest<T>(`auth/${path}`, options);
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
  if (error.code === "file_too_large") return "Файл должен быть не больше 20 МБ.";
  if (error.code === "invalid_upload") return "Выберите один файл для загрузки.";
  if (error.code === "avatar_too_large") return "Изображение должно быть не больше 5 МБ.";
  if (error.code === "invalid_avatar") return "Выберите корректное изображение JPEG или PNG: до 4096 пикселей по стороне и до 16 мегапикселей.";
  if (error.code === "invalid_name") return "Имя должно содержать от 1 до 100 символов без управляющих символов.";
  if (error.status === 401) return "Неверный email или пароль.";
  if (error.status === 409) return "Этот email уже зарегистрирован. Войдите в аккаунт.";
  if (error.status === 429) return "Слишком много попыток. Подождите минуту и попробуйте снова.";
  if (error.status === 400) return "Проверьте email и пароль: пароль должен содержать от 8 до 128 символов.";
  if (error.code === "unsupported_browser") return "Откройте приложение в современном браузере через localhost или HTTPS.";
  return "Сервис временно недоступен. Проверьте подключение и попробуйте снова.";
}
