# Uptime App

Основа сервиса мониторинга доступности. Backend реализует регистрацию и авторизацию через Go API и PostgreSQL; мониторинг и интеграция с frontend пока не реализованы.

```text
uptime-app/
├── backend/          # Go, GORM, JWT, Goose SQL-миграции
├── frontend/         # Next.js: минимальное приложение
├── compose.yaml      # API и постоянная PostgreSQL 18
├── compose.test.yaml # Изолированная PostgreSQL и Go-тесты
└── docs/auth-plan.md # План и чек-лист реализации
```

Для backend нужны Docker и Docker Compose. Подготовьте `backend/.env` по [инструкции backend](backend/README.md), затем из корня:

```sh
docker compose --env-file backend/.env up --build -d
```

В текущем локальном `backend/.env` задан `API_PORT=8081`: API доступен на `http://localhost:8081`, проверка готовности — `http://localhost:8081/healthz`. Внутри контейнера Go-сервер слушает `8080`; Compose перенаправляет внешний `API_PORT` на этот порт. Значение `API_PORT` по умолчанию в Compose и `backend/.env.example` — `8080`. Для текущего локального запуска задайте `API_PORT=8081` в `backend/.env`; если порт занят, выберите другой свободный порт. PostgreSQL не публикует порт на хост и сохраняет данные в volume.

Frontend запускается отдельно, см. [frontend/README.md](frontend/README.md). Форм авторизации в нём пока нет. Контракт API, примеры запросов и команды проверок описаны в [backend/README.md](backend/README.md).
