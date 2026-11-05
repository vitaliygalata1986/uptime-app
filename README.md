# Uptime App

Основа сервиса мониторинга доступности. Backend реализует регистрацию и авторизацию через Go API и PostgreSQL; frontend реализует регистрацию, вход и кабинет. Мониторинг пока не реализован.

```text
uptime-app/
├── backend/          # Go, GORM, JWT, Goose SQL-миграции
├── frontend/         # Next.js: регистрация, вход и кабинет
├── compose.yaml      # API и постоянная PostgreSQL 18
├── compose.test.yaml # Изолированная PostgreSQL и Go-тесты
└── docs/auth-plan.md # План и чек-лист реализации
```

Для backend нужны Docker и Docker Compose. Подготовьте `backend/.env` по [инструкции backend](backend/README.md), затем из корня:

```sh
docker compose --env-file backend/.env up --build -d
```

В текущем локальном `backend/.env` задан `API_PORT=8081`: API доступен на `http://localhost:8081`, проверка готовности — `http://localhost:8081/healthz`. Внутри контейнера Go-сервер слушает `8080`; Compose перенаправляет внешний `API_PORT` на этот порт. Значение `API_PORT` по умолчанию в Compose и `backend/.env.example` — `8080`. Для текущего локального запуска задайте `API_PORT=8081` в `backend/.env`; если порт занят, выберите другой свободный порт. PostgreSQL не публикует порт на хост и сохраняет данные в volume.

Frontend запускается отдельно, см. [frontend/README.md](frontend/README.md). В нём доступны регистрация, вход и личный кабинет. Контракт API, примеры запросов и команды проверок описаны в [backend/README.md](backend/README.md).

HTTP-контроллеры backend расположены в своих модулях (`internal/auth/handler.go`, `internal/health/handler.go`); `internal/httpapi/` собирает маршруты и выполняет общие HTTP-проверки, CORS и IP-лимиты. Подробная структура описана в [backend/README.md](backend/README.md).

## Общий запуск

Нужны Node.js 22.13+ или 24+, Docker Compose и подготовленный `backend/.env`. Установите зависимости: `npm --prefix frontend ci`. Из этой папки:

```sh
npm run dev
```

Скрипт поднимает PostgreSQL и backend, ждёт `/healthz`, затем запускает frontend на `http://localhost:3000`. Адрес API определяется по опубликованному порту Compose (локально `8081`). `NEXT_PUBLIC_API_URL` переопределяет адрес. Для другого frontend-порта задайте `FRONTEND_PORT` и добавьте соответствующий Origin в `ALLOWED_ORIGINS` backend. Для локального HTTP требуется `LOCAL_HTTP=true`.

Ctrl+C останавливает frontend. Backend остаётся запущен; остановить его без удаления данных: `npm run dev:stop`.
