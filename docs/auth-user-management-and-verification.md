# Проверка микросервисов + документация по авторизации и управлению пользователями

Дата проверки: **25 февраля 2026**  
Контур: **локальный dev** (Redis + Postgres в Docker, `user-service`/`auth-service`/`api-gateway` запущены локально)

## 1. Результаты проверки работоспособности

### 1.1 Go-тесты

| Команда | Результат | Комментарий |
|---|---|---|
| `cd proto && go test ./...` | ✅ | PASS |
| `cd services/auth-service && go test ./...` | ✅ | PASS |
| `cd services/user-service && go test ./...` | ✅ | PASS |
| `cd services/api-gateway && go test ./...` | ✅ | PASS |
| `cd services/notification-service && go test ./...` | ✅ | PASS |
| `cd services/post-service && go test ./...` | ✅ | PASS (после `go mod tidy`) |

### 1.2 Проверка Docker Compose

После фиксов выполнено:
- `docker compose config --services` — успешно;
- `docker compose build auth-service user-service post-service api-gateway` — успешно;
- `docker compose up -d` — стек поднимается, gateway health возвращает `200 healthy`.

Что изменено:
- для `auth-service`, `user-service`, `post-service`, `api-gateway` `build.context` переведен на корень репозитория;
- Dockerfile этих сервисов обновлены: в builder копируется `proto` + код конкретного сервиса;
- устаревшее поле `version` удалено из `docker-compose.yml`.

### 1.3 Live-проверка взаимодействия через API Gateway

Проверено на поднятых `redis` + `postgres_user` и локально запущенных `user-service`, `auth-service`, `api-gateway`.

| Сценарий | Результат | Статус |
|---|---|---|
| `GET /health` | gateway отвечает `200 healthy` | ✅ |
| `POST /api/v1/auth/register` | регистрация пользователя | ✅ |
| `POST /api/v1/auth/login` | логин валидного пользователя | ✅ |
| `POST /api/v1/auth/login` (неверный пароль) | `401 INVALID_CREDENTIALS` | ✅ |
| `GET /api/v1/users` без токена | `401 MISSING_TOKEN` | ✅ |
| `GET /api/v1/users/:id` с токеном | профиль возвращается | ✅ |
| `PUT /api/v1/users/:id` (свой id) | обновление профиля | ✅ |
| `PUT /api/v1/users/:id` (чужой id) | `403 UPDATE_FAILED` | ✅ (блокируется) |
| `GET /api/v1/users` с токеном | список пользователей | ✅ |
| `GET /api/v1/public/users/*` | публичный поиск/статистика/профиль | ✅ |
| `POST /api/v1/auth/refresh` | refresh проходит | ✅ |
| повторный refresh старым refresh token | `401 REFRESH_FAILED` | ✅ (rotation/revoke работает) |
| `POST /api/v1/auth/logout` + повторная `validate` | токен отозван (`401`) | ✅ |
| `GET /api/v1/auth/google` (web) | auth URL генерируется | ✅ |
| `GET /api/v1/auth/google` (mobile без PKCE) | `400 INVALID_REQUEST` / PKCE required | ✅ |
| `GET /api/v1/auth/google/callback?state=fake&code=fake` | `401 INVALID_CALLBACK` | ✅ |
| `POST /api/v1/auth/exchange` с fake code | `401 EXCHANGE_FAILED` | ✅ |

Итог: **цепочка авторизации и user-management через Gateway рабочая**, ключевые блокеры сборки/интеграции устранены.

---

## 2. Как устроена авторизация (текущее поведение)

### 2.1 Компоненты и роли

- **API Gateway**: HTTP входная точка, проверка bearer-токена, проксирование в gRPC (`services/api-gateway/internal/routes/routes.go:29-101`).
- **Auth Service**: выдача/проверка JWT, refresh/logout, OAuth state/auth-code flow (`services/auth-service/internal/application/services/auth_service.go`).
- **User Service**: создание/поиск/обновление пользователей + проверка email/password (`services/user-service/internal/application/services/user_service.go`).
- **Redis**: хранение access/refresh токенов, blacklist, OAuth state, temporary auth_code (`services/auth-service/internal/infrastructure/redis/token_repository.go`).
- **Postgres userdb**: пользовательские данные (`services/user-service/internal/infrastructure/postgres/user_repository.go`).

### 2.2 Маршруты авторизации в Gateway

- Публичные:
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `GET /api/v1/auth/google`
  - `GET /api/v1/auth/google/callback`
  - `POST /api/v1/auth/exchange`
  - `POST /api/v1/auth/refresh`
  - `POST /api/v1/auth/logout` (без `AuthMiddleware`: access-токен может быть истёкшим)
- Защищенные (через `AuthMiddleware`):
  - `GET /api/v1/auth/validate`

Источник: `services/api-gateway/internal/routes/routes.go:32-53`.

### 2.3 Email/password flow

1. Клиент вызывает `register`/`login` на Gateway.
2. Gateway делает gRPC вызов в Auth Service (`services/api-gateway/internal/clients/auth_client.go`).
3. Auth Service:
   - `register`: создаёт пользователя через User Service gRPC (`auth_service.go:313-357`).
   - `login`: валидирует пароль через User Service gRPC (`auth_service.go:361-405`).
4. Auth Service генерирует access/refresh JWT и сохраняет оба в Redis (`auth_service.go:437-490`).
5. Gateway возвращает access token и user payload. Refresh token при `AUTH_REFRESH_TOKEN_COOKIE=true` уходит только в HttpOnly cookie и в JSON его нет; при выключенном режиме он в JSON (`buildAuthResponse`, `withoutCookieToken` в `auth_handler.go`). Так же отвечают `exchange` и `refresh`.

### 2.4 Проверка access token

- `AuthMiddleware` в Gateway берёт `Authorization: Bearer ...`, вызывает `ValidateToken` в Auth Service (`services/api-gateway/internal/middleware/auth.go:14-51`).
- Auth Service проверяет:
  - не в blacklist (`auth_service.go:409-416`);
  - JWT валиден и тип `access` (`auth_service.go:418-424`);
  - токен присутствует в Redis (`auth_service.go:426-428`).

### 2.5 Refresh token

1. Клиент вызывает `POST /api/v1/auth/refresh`.
2. Gateway берёт refresh token из cookie (если включено) или JSON body (`auth_handler.go:307-326`).
3. Auth Service проверяет тип `refresh`, наличие в Redis, blacklist (`auth_service.go:223-254`).
4. Выдаёт новую пару токенов, старый refresh добавляет в blacklist (`auth_service.go:267-287`).
5. Неудачный refresh с `401` очищает refresh cookie на Gateway; ошибка бэкенда (`5xx`) cookie не трогает. BFF при `401`/`400` чистит и свои cookies.

### 2.6 Logout

- Gateway берёт access token из `Authorization` (может быть истёкшим) и refresh token из cookie или JSON body и передаёт оба в `Logout`. Refresh cookie очищается при любом исходе; без обоих токенов ответ `200` без вызова Auth Service.
- Auth Service берёт пользователя из валидного access token, иначе из валидного refresh token, и отзывает все его токены: `DeleteUserTokens` удаляет их из Redis и пишет `auth:blacklist:<token>` для каждого access token на остаток его TTL, так что они перестают работать и на внутренних HTTP API user/post/notification.
- BFF вызывает logout, если есть access или refresh cookie, и чистит свои cookies при любом исходе, включая `429` от лимитера.

### 2.6.1 Удаление аккаунта

- `DELETE /api/v1/users/:id` со своим id: Gateway по порядку вызывает `UnpublishUserPosts` в Post Service (посты автора сняты с публикации и убраны из поиска, строки остаются), `Logout` с токеном запроса (отзыв всех сессий, как выше) и `DeleteUser` в User Service. Шаг не удался — ответ с ошибкой (`5xx`; от `Logout` возможен `401`, если токен истёк между проверкой и отзывом), следующие шаги не выполняются, аккаунт не удалён; запрос можно повторить, скрытие постов идемпотентно. Известный предел: пост, опубликованный из другой сессии в миллисекунды между скрытием постов и отзывом сессий, останется открытым. Посты идут первыми: если скрыть их не удалось (Post Service или его база недоступны), пользователь ещё в системе и может повторить. От Kafka этот шаг не зависит: удаление постов из поиска ставится в очередь в той же транзакции и уходит, когда Kafka доступна. Чужой id ничего из этого не вызывает, User Service отвечает `403`.

### 2.7 Google OAuth (web + mobile)

Flow реализован как secure auth-code exchange:

1. `GET /api/v1/auth/google`:
   - определение платформы (web/mobile);
   - проверка redirect URI allowlist;
   - для mobile обязателен PKCE challenge;
   - генерация `state`, сохранение в Redis (10 мин);
   - возврат Google Auth URL.  
   Код: `auth_service.go:73-124`, `550-616`.

2. `GET /api/v1/auth/google/callback`:
   - чтение и одноразовое удаление `state` из Redis;
   - обмен `code` у Google;
   - upsert/получение пользователя через User Service;
   - выпуск temporary `auth_code` (5 мин) в Redis;
   - redirect на клиентский `redirect_uri?auth_code=...&state=...`.  
   Код: `auth_service.go:127-180`, `token_repository.go:61-91`.

3. `POST /api/v1/auth/exchange`:
   - одноразовый `auth_code` + PKCE verifier;
   - выдача JWT пары (access/refresh).  
   Код: `auth_service.go:183-221`, `618+`.

---

## 3. Как устроено управление пользователями

### 3.1 Маршруты пользователей в Gateway

- Публичные:
  - `GET /api/v1/public/users/search`
  - `GET /api/v1/public/users/stats`
  - `GET /api/v1/public/users/:id/profile`
- Защищенные:
  - `POST /api/v1/users`
  - `GET /api/v1/users`
  - `GET /api/v1/users/:id`
  - `PUT /api/v1/users/:id`
  - `DELETE /api/v1/users/:id`

Источник: `services/api-gateway/internal/routes/routes.go:60-90`.

### 3.2 Контроль прав

- Gateway извлекает `userID` из валидного access token и передает как `actor_id` в gRPC update/delete (`services/api-gateway/internal/handlers/user_handler.go:83-96`, `106-111`).
- User Service gRPC сервер строго проверяет `actor_id == id` (`services/user-service/internal/interfaces/grpc/server.go:90-93`, `126-129`).
- Это предотвращает изменение/удаление чужого профиля.

### 3.3 Создание и хранение пользователя

- User создается в User Service (`user_service.go:30-83`).
- Пароль (если передан) хешируется `bcrypt` (`user_service.go:52-59`).
- Удаление мягкое (`is_active=false`) в репозитории (`user_repository.go`).

---

## 4. Модель безопасности (актуально сейчас)

- JWT разделены по типам: `access` и `refresh` (`auth_service.go:441-455`).
- Токены state/auth_code одноразовые через `GETDEL` в Redis (`token_repository.go:261-269`).
- Защищенные маршруты Gateway централизованы через `AuthMiddleware`.
- Есть rate limiting на Gateway через Redis (`services/api-gateway/internal/middleware/rate_limiter.go`).
- Перебор пароля ограничен и по аккаунту: 10 неудачных входов за 15 минут закрывают вход по паролю для этого email до конца окна (`TOO_MANY_LOGIN_ATTEMPTS`, gRPC `ResourceExhausted`). Подробности и цена — `docs/production-security.md`, раздел Sign-In Protection.
- `GOOGLE_ALLOWED_DOMAINS` сверяется с `hd` профиля Google (домен Workspace), а не с окончанием email, и проверяется до создания пользователя.
- CORS управляется через env (`CORS_ALLOWED_*`, `CORS_EXPOSE_HEADERS`, `CORS_ALLOW_CREDENTIALS`).
- Опционально поддерживается refresh-token cookie (`AUTH_REFRESH_TOKEN_COOKIE`) с явным `SameSite` (`AUTH_REFRESH_TOKEN_COOKIE_SAMESITE`, по умолчанию `Lax`), `HttpOnly`, `Secure` в prod (`auth_handler.go`).

---

## 5. Критичные замечания и риски

1. **Отзыв сессий держится только на индексе токенов**  
   `DeleteUserTokens` читает set `auth:user_tokens:<id>` и удаляет перечисленные ключи; keyspace не сканируется (logout публичный, `SCAN` мог бы запустить кто угодно с валидным токеном). Каждый токен попадает в индекс при записи, а индекс живёт refresh TTL и продлевается при каждой записи и ротации, поэтому переживает свои токены. Токен, записанный в Redis в обход `storeToken`, отзыв не увидит.

---

## 6. Мини-runbook для ручной проверки

1. Поднять инфраструктуру:
   - `docker compose up -d redis postgres_user`
2. Запустить локально:
   - `services/user-service` (`go run .`)
   - `services/auth-service` (`REDIS_URL=localhost:6379 go run .`)
   - `services/api-gateway` (`REDIS_URL=localhost:6379 go run .`)
3. Smoke-тесты:
   - `GET /health`
   - `POST /api/v1/auth/register`
   - `POST /api/v1/auth/login`
   - `GET /api/v1/users` (без/с токеном)
   - `PUT /api/v1/users/:id` (свой/чужой id)
   - `POST /api/v1/auth/refresh`
   - `POST /api/v1/auth/logout`
   - `GET /api/v1/auth/google` (web/mobile + PKCE)
