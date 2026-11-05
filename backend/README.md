# Backend

Go 1.27 API на `net/http` с PostgreSQL 18, GORM, Goose и JWT. Реализованы регистрация, вход, обновление токенов, выход и текущий пользователь. Frontend авторизации реализован в `../frontend/`. Мониторинг, подтверждение email и восстановление пароля пока не реализованы.

```text
backend/
├── cmd/api/main.go           # Конфигурация → БД → миграции → HTTP
├── internal/config/          # Чтение и проверка окружения
├── internal/store/           # GORM-модели, запросы и транзакции
│   └── migrations/           # Встроенные в бинарник SQL-миграции Goose
├── internal/auth/            # Пароли, JWT, сессии, DTO и HTTP-контроллеры авторизации
│   └── handler.go            # Register, Login, Refresh, Logout, Me, cookie и ошибки auth
├── internal/health/          # HTTP-контроллер готовности БД (/healthz)
├── internal/httpapi/         # Сборка маршрутов, общие HTTP-проверки, CORS и лимиты
├── internal/httpx/           # Общие функции чтения JSON и записи ответов/ошибок
└── Dockerfile                # Go 1.27.3, запуск от UID 10001
```

HTTP-контроллеры находятся в своих модулях: `auth` обрабатывает авторизацию, `health` — готовность БД. `cmd/api/main.go` создаёт сервисы и контроллеры и передаёт их в `httpapi.New`, где маршруты связываются с обработчиками и лимитами. Общие проверки метода, Origin, Content-Type, CORS и IP-лимитов выполняются до вызова контроллера. Модули используют независимый пакет `httpx` и не импортируют `httpapi`.

При добавлении модуля размещайте его HTTP-контроллер в `internal/<module>/handler.go`, создавайте зависимости в `cmd/api/` и подключайте маршруты в `internal/httpapi/`. Тесты контроллера размещайте рядом с ним; общие HTTP-проверки и интеграционные сценарии API остаются в `internal/httpapi/`.

## Подготовка и запуск

Следующие команды выполняются из `backend/`. Системный Go для Docker-запуска не нужен.

Создайте локальный `.env` со случайными секретами (команда откажется перезаписывать существующий файл):

```sh
python3 - <<'PY'
import base64, os, secrets
from pathlib import Path
text = Path('.env.example').read_text()
text = text.replace('POSTGRES_PASSWORD=\n', 'POSTGRES_PASSWORD=' + secrets.token_hex(32) + '\n')
text = text.replace('JWT_SECRET_BASE64=\n', 'JWT_SECRET_BASE64=' + base64.b64encode(secrets.token_bytes(48)).decode() + '\n')
fd = os.open('.env', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, 'w') as f:
    f.write(text)
PY
```

В текущем локальном `.env` задан `API_PORT=8081`. После создания `.env` из шаблона задайте в нём `API_PORT=8081`, чтобы использовать те же адреса и примеры запросов, затем запустите стек:

```sh
docker compose -f ../compose.yaml --env-file .env up --build -d
```

API: `http://localhost:8081`, проверка готовности: `http://localhost:8081/healthz`. Внутри контейнера Go-сервер слушает `8080`; Compose публикует его на внешнем `API_PORT`. Значение по умолчанию в Compose и `.env.example` — `8080`. При занятом внешнем порте измените `API_PORT` в `.env` и адреса запросов. БД доступна только в сети Compose. Данные хранятся в `postgres_data` с точкой монтирования `/var/lib/postgresql`, подходящей для PostgreSQL 18.

```sh
docker compose -f ../compose.yaml --env-file .env logs --tail=50 api
docker compose -f ../compose.yaml --env-file .env stop
docker compose -f ../compose.yaml --env-file .env up -d
```

`stop` и `down` сохраняют постоянный volume; `down -v` удаляет данные. Реальный `.env` игнорируется Git и исключён из Docker-контекста.

| Переменная | Назначение |
| --- | --- |
| `POSTGRES_USER`, `POSTGRES_DB` | Пользователь и БД Compose; по умолчанию `uptime` |
| `POSTGRES_PASSWORD` | Случайный пароль; генератор использует hex, безопасный внутри URL подключения |
| `JWT_SECRET_BASE64` | Стандартный Base64 от минимум 32 случайных байтов; неверная кодировка или длина блокируют запуск |
| `JWT_ISSUER`, `JWT_AUDIENCE` | Обязательные issuer и audience JWT |
| `ALLOWED_ORIGINS` | Точные origin через запятую, без пути и завершающего `/`; локально `http://localhost:3000` |
| `LOCAL_HTTP` | Явный локальный режим; по умолчанию `false`. Только `true` разрешает HTTP-origin и cookie без `Secure` |
| `API_PORT` | Публикуемый на loopback порт; в текущем локальном `.env` — `8081`, по умолчанию в Compose и `.env.example` — `8080` |
| `HTTP_ADDR` | Адрес Go HTTP-сервера; Compose задаёт `:8080` независимо от `API_PORT` |
| `DATABASE_URL` | URL PostgreSQL для запуска бинарника; Compose собирает его из переменных БД |

Вне локального режима используйте HTTPS-origin и `LOCAL_HTTP=false`: refresh-cookie получает `Secure`. TLS завершается перед API. Используйте новые секреты для каждого окружения. При изменении пароля уже созданной БД одного редактирования `.env` недостаточно: существующий пароль PostgreSQL сам не изменяется.

Для запуска без контейнера нужны Go 1.27+ и доступная PostgreSQL. Экспортируйте настройки из таблицы, включая `DATABASE_URL`, и выполните `go run ./cmd/api`. Приложение само не читает `.env`: его обрабатывает Compose.

## API и сроки жизни

| Метод | Маршрут | Результат |
| --- | --- | --- |
| POST | `/api/auth/register` | `201`, пользователь и новая сессия |
| POST | `/api/auth/login` | `200`, отдельная сессия устройства |
| POST | `/api/auth/refresh` | `200`, новая пара токенов |
| POST | `/api/auth/logout` | `204`, отзыв сессии и удаление cookie; повторный выход допустим |
| GET | `/api/auth/me` | `200`, `{id, email, created_at}` по access JWT |
| GET | `/healthz` | `200`, если HTTP и соединение с БД готовы |

Все POST-запросы требуют разрешённый `Origin` и `Content-Type: application/json`. Регистрация и вход принимают `{email, password}`, refresh и logout — `{}`. Тело ограничено 8 KiB; неизвестные поля и лишние JSON-значения запрещены. Пароль длиной 8–128 Unicode-символов сохраняется без обрезки и нормализации. Email очищается от пробелов по краям и приводится к нижнему регистру.

Регистрация, вход и refresh возвращают:

```json
{
  "user": {"id": 1, "email": "user@example.com", "created_at": "2026-10-30T12:00:00Z"},
  "access_token": "<JWT>",
  "token_type": "Bearer",
  "expires_in": 900
}
```

Access JWT действует 15 минут; алгоритм HS256, проверяются подпись, алгоритм, срок, issuer и audience. Refresh-сессия действует строго 24 часа от входа, ротация не продлевает срок. Refresh передаётся только cookie `refresh_token` с `HttpOnly`, `SameSite=Lax`, `Path=/api/auth`, без `Domain`; срок cookie равен остатку сессии. После logout access JWT действует до своего истечения.

В БД хранятся Argon2id-хеши паролей (19 MiB, 2 итерации, параллелизм 1, индивидуальная соль) и SHA-256-хеши refresh из 32 случайных байтов. Ротация блокирует строку сессии через `SELECT … FOR UPDATE`, затем повторно проверяет токен. Повторное использование погашенного refresh отзывает всю соответствующую сессию, сохраняя остальные устройства. Два одновременных refresh с одним токеном приводят к одному успеху и одному отказу с отзывом сессии: клиенту следует сериализовать обновления.

Ошибки имеют форму `{"error":{"code":"unauthorized","message":"Invalid credentials or token"}}`. Статусы: `400` — некорректные данные, `401` — учётные данные/токен, `403` — Origin, `409` — занятый email, `429` — лимит. Ошибка входа одинакова для неизвестного email и неверного пароля. Ответы авторизации имеют `Cache-Control: no-store`.

Лимиты на IP в минуту: регистрация 5, вход 10, refresh 30. `429` содержит `Retry-After`. Лимитер хранится в памяти одного процесса и очищает устаревшие записи при запросах. Заголовки `X-Forwarded-For`, `X-Real-IP` не используются: за прокси запросы учитываются по IP прокси.

## HTTP-пример с cookie-файлом

Команды из `backend/`, для текущего локального `API_PORT=8081`; при другом значении измените `API_URL`:

```sh
API_URL=http://localhost:8081
curl -sS -c /tmp/uptime.cookies -o /tmp/uptime-auth.json \
  -H 'Origin: http://localhost:3000' -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"a long example password"}' \
  "$API_URL/api/auth/register"
ACCESS_TOKEN=$(python3 -c 'import json; print(json.load(open("/tmp/uptime-auth.json"))["access_token"])')
curl -sS -H "Authorization: Bearer $ACCESS_TOKEN" "$API_URL/api/auth/me"
curl -sS -b /tmp/uptime.cookies -c /tmp/uptime.cookies -o /tmp/uptime-auth.json \
  -H 'Origin: http://localhost:3000' -H 'Content-Type: application/json' \
  -d '{}' "$API_URL/api/auth/refresh"
curl -sS -i -b /tmp/uptime.cookies -c /tmp/uptime.cookies \
  -H 'Origin: http://localhost:3000' -H 'Content-Type: application/json' \
  -d '{}' "$API_URL/api/auth/logout"
# 401: cookie удалена, обновление невозможно.
curl -sS -i -b /tmp/uptime.cookies \
  -H 'Origin: http://localhost:3000' -H 'Content-Type: application/json' \
  -d '{}' "$API_URL/api/auth/refresh"
# Вход создаёт новую независимую сессию.
curl -sS -c /tmp/uptime.cookies \
  -H 'Origin: http://localhost:3000' -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"a long example password"}' \
  "$API_URL/api/auth/login"
```

## Миграции и проверки

Goose применяет встроенные SQL-миграции до запуска HTTP; при ошибке API не запускается. GORM и Goose используют общий `*sql.DB`, `AutoMigrate` не вызывается. Изменения схемы добавляются новыми версионированными файлами в `internal/store/migrations/`; применённые файлы не редактируются. Повторный запуск не применяет выполненные версии заново.

Проверки в Docker с отдельной PostgreSQL (из `backend/`):

```sh
docker compose -p uptime-auth-tests -f ../compose.test.yaml up \
  --build --abort-on-container-exit --exit-code-from tests
docker compose -p uptime-auth-tests -f ../compose.test.yaml down
```

Тестовая БД не публикует порт и использует `tmpfs`, отдельно от данных приложения. Контейнер тестов выполняет сборку, `go vet ./...`, `go test -race -count=1 -v ./...` в Go 1.27.3. Каждый интеграционный тест создаёт свою случайную схему и удаляет её после проверки; тестовому пользователю нужны права `CREATE SCHEMA`.

Для локального Go 1.27+:

```sh
go build -o bin/api ./cmd/api
go vet ./...
go test -race ./...
```

Без `TEST_DATABASE_URL` интеграционные тесты явно пропускаются. Для их выполнения задайте URL **отдельной тестовой PostgreSQL** и используйте `go test -race -count=1 ./...`.

Unit-тесты проверяют конфигурацию, пароли, JWT, валидацию, cookie, CORS/Origin и лимиты. Интеграционные тесты с настоящей БД проверяют миграции, атомарность, конфликт email, конкурентную регистрацию, вход, `/me`, ротацию, повторное использование и конкурентный refresh, фиксированные 24 часа, независимость устройств и logout. HTTP-сценарий через опубликованный порт и сохранение данных после перезапуска контейнеров проверяются отдельно от unit-тестов.

Повторяемая HTTP-проверка с Python 3 для текущего локального порта (секреты не печатаются). Явно передавайте `--url`: без него скрипт использует стандартный порт `8080`.

```sh
python3 scripts/http_smoke.py --url http://localhost:8081
```

Проверка сохранения пользователя и refresh-сессии после пересоздания контейнеров, с сохранением постоянного volume:

```sh
STATE_DIR=$(mktemp -d /tmp/uptime-auth-smoke-XXXXXX)
python3 scripts/http_smoke.py --url http://localhost:8081 --phase prepare --state-dir "$STATE_DIR"
docker compose -f ../compose.yaml --env-file .env down
docker compose -f ../compose.yaml --env-file .env up -d
# Дождитесь успешного GET /healthz, затем:
python3 scripts/http_smoke.py --url http://localhost:8081 --phase verify --state-dir "$STATE_DIR"
```

Скрипт использует cookie-файл с правами `0600`. Фаза `verify` проверяет того же пользователя и сохранённую сессию, выполняет logout и удаляет временные файлы токенов. Выполняйте вторую фазу в пределах 15 минут, пока сохранённый access JWT действителен.
