# Payment Service

Микросервис обработки платежей. Поддерживает оплату через **T-Bank** (банковские карты)
и **Telegram Stars**.

## Архитектура

```
cmd/payment/main.go          — точка входа, DI-контейнер
├── internal/config/         — конфигурация (yaml + env)
├── internal/domain/         — доменные модели и типы
├── internal/service/        — бизнес-логика платежей
├── internal/server/grpc/    — gRPC сервер и хендлеры
│   ├── app/                 — инициализация gRPC сервера
│   └── handlers/            — gRPC хендлеры (TbankInit, TGInit, Cancel, Webhook...)
├── internal/providers/      — провайдеры платежей
│   ├── tbank/               — T-Bank (инициализация, отмена, статус, вебхуки)
│   └── telegram/            — Telegram Stars (init, precheckout, cancel, successful)
├── internal/repository/postgres/ — слой доступа к БД (PGX)
├── internal/kafka/producer/ — Kafka-producer для отправки событий в balance
├── internal/workers/        — фоновые воркеры
│   ├── outbox_workers/      — воркер отправки отложенных сообщений из outbox-таблицы
│   └── currency_rates/      — воркер синхронизации курсов валют (ЦБ РФ)
├── internal/middleware/grpc/ — gRPC middleware (error, logging, panic recovery)
├── internal/telemetry/      — OpenTelemetry метрики
├── internal/app_error/      — кастомные ошибки приложения
├── migrations/              — миграции БД (goose)
└── configs/main.yml         — конфигурационный файл
```

## Основные возможности

- Инициализация платежей через **T-Bank** (карты) и **Telegram Stars**
- Обработка вебхуков от провайдеров (подтверждение, отмена, refund)
- Идемпотентность по ключу (`idempotency_key` уникален)
- Жизненный цикл платежа: `pending` → `started` → `processed` → `completed` / `failed` / `cancelled` → `refunded`
- Outbox-паттерн для гарантированной доставки событий в balance-сервис через Kafka
- Автоматическая синхронизация курсов валют (USD, EUR) с API ЦБ РФ
- Long polling статусов платежей (ожидание изменения статуса клиентом без WebSocket)
- OpenTelemetry (трассировка, метрики, логирование)

## Стек технологий

| Компонент        | Технология                  |
|------------------|-----------------------------|
| Язык             | Go 1.25                     |
| Транспорт        | gRPC (port 44050)           |
| База данных      | PostgreSQL 16               |
| Миграции         | goose                       |
| Брокер сообщений | Kafka (Sarama)              |
| Телеметрия       | OpenTelemetry (OTLP gRPC)   |
| Провайдеры       | T-Bank API, Telegram Bot API|
| Курсы валют      | ЦБ РФ (cbr-xml-daily.ru)    |
| Контейнеризация  | Docker, docker-compose      |
| CI/CD            | GitHub Actions, GHCR        |

## Переменные окружения

Перед запуском необходимо расшифровать `.env.vault` и заполнить секретные поля:

```bash
# Расшифровка .env файла
ansible-vault view .env.vault > .env
# Пароль: 7d28a604-bb66-4ba9-a1ed-7679a2f82001
```

В полученном `.env` файле **необходимо заполнить** свои данные:

```bash
TINKOFF_TERMINAL_KEY=<ваш terminal key от T-Bank>
TINKOFF_PASSWORD=<ваш пароль от T-Bank>
TELEGRAM_BOT_TOKEN=<токен вашего Telegram бота>
```

Остальные переменные уже заполнены значениями по умолчанию.

### Полный список переменных окружения

| Переменная                    | Описание                             | По умолчанию                  |
|-------------------------------|--------------------------------------|-------------------------------|
| `ENV`                         | Окружение (development/stage/prod)   | `development`                 |
| `POSTGRES_HOST`               | Хост PostgreSQL                      | `postgres`                    |
| `POSTGRES_PORT`               | Порт PostgreSQL                      | `5432`                        |
| `POSTGRES_USER`               | Пользователь БД                      | `payments_user`               |
| `POSTGRES_PASSWORD`           | Пароль БД                            | `payments_pass`               |
| `POSTGRES_DB`                 | Имя БД                               | `payments`                    |
| `GRPC_HOST`                   | Хост gRPC сервера                    | `0.0.0.0`                     |
| `GRPC_PORT`                   | Порт gRPC сервера                    | `44050`                       |
| `KAFKA_BROKERS`               | Список брокеров Kafka (через запятую)|                               |
| `TINKOFF_TERMINAL_KEY`        | Terminal Key T-Bank                  | **обязательно**               |
| `TINKOFF_PASSWORD`            | Пароль T-Bank                        | **обязательно**               |
| `TINKOFF_BASE_URL`            | Базовый URL T-Bank API               | `https://securepay.tinkoff.ru/v2`|
| `TINKOFF_NOTIFICATION_URL`    | URL для вебхуков T-Bank              |                               |
| `TELEGRAM_BOT_TOKEN`          | Токен Telegram бота                  | **обязательно**               |
| `TELEGRAM_BASE_URL`           | Базовый URL Telegram API             | `https://api.telegram.org/bot`|

## Быстрый старт (локально)

```bash
# 1. Расшифровать .env
ansible-vault view .env.vault > .env
# Пароль: 7d28a604-bb66-4ba9-a1ed-7679a2f82001

# 2. Заполнить секретные поля в .env:
#    TINKOFF_TERMINAL_KEY, TINKOFF_PASSWORD, TELEGRAM_BOT_TOKEN

# 3. Запустить сервис
make docker-prod

# 4. Проверить
grpcurl -plaintext localhost:44050 list
```

## Тестирование

```bash
# Юнит-тесты
make test

# Интеграционные тесты (требуют Docker)
make docker-test-up    # поднять test-postgres
make test-integration  # запустить тесты с тегом integration
make docker-test-down  # остановить контейнеры

# E2E тесты (требуют реальные ключи T-Bank и Telegram)
# Заполнить: TINKOFF_TERMINAL_KEY, TINKOFF_PASSWORD, TELEGRAM_BOT_TOKEN
make test-e2e
```

## База данных

### Таблица `payments`

| Колонка               | Тип           | Constraints   | Описание                                                                       |
| --------------------- | ------------- | ------------- | ------------------------------------------------------------------------------ |
| `id`                  | `UUID`        | `PRIMARY KEY` | Уникальный идентификатор платежа                                               |
| `user_id`             | `UUID`        | `NOT NULL`    | ID пользователя                                                                |
| `idempotency_key`     | `VARCHAR`     | `UNIQUE`      | Ключ идемпотентности, защита от дублирования                                   |
| `amount`              | `INTEGER`     | `NOT NULL`    | Сумма в минимальных единицах (копейки для RUB, stars для XTR)                  |
| `currency`            | `VARCHAR(3)`  | `NOT NULL`    | Код валюты (`RUB`, `XTR`)                                                      |
| `status`              | `VARCHAR`     | `NOT NULL`    | Статус платежа                                                                 |
| `provider_name`       | `VARCHAR`     | `NOT NULL`    | Провайдер (`tbank_form`, `tg_stars`)                                           |
| `provider_payment_id` | `VARCHAR`     | —             | ID платежа у провайдера                                                        |
| `provider_user_id`    | `BIGINT`      | —             | ID плательщика у провайдера                                                    |
| `payment_url`         | `TEXT`        | —             | URL для оплаты                                                                 |
| `description`         | `TEXT`        | —             | Описание платежа                                                               |
| `expires_at`          | `TIMESTAMPTZ` | —             | Срок действия                                                                  |
| `paid_at`             | `TIMESTAMPTZ` | —             | Время оплаты                                                                   |
| `created_at`          | `TIMESTAMPTZ` | `NOT NULL`    | Время создания                                                                 |
| `updated_at`          | `TIMESTAMPTZ` | `NOT NULL`    | Время последнего обновления                                                    |

**Индексы:**
- `UNIQUE (idempotency_key)` — гарантирует идемпотентность на уровне БД
- `INDEX (provider_name, provider_payment_id)` — быстрый поиск при обработке вебхуков

### Таблица `payments_outbox`

Outbox-таблица для гарантированной отправки событий в balance-сервис через Kafka.

| Колонка       | Тип       | Описание                              |
|---------------|-----------|---------------------------------------|
| `operation_id`| `VARCHAR` | Уникальный ID операции                |
| `payment_id`  | `UUID`    | ID платежа                            |
| `type`        | `VARCHAR` | Тип операции (`deposit`, `refund`)    |
| `amount`      | `INTEGER` | Сумма                                 |
| `event_key`   | `VARCHAR` | Ключ события для партицирования Kafka |
| `payload`     | `JSONB`   | Полные данные события                 |
| `status`      | `VARCHAR` | Статус (`pending`, `processed`, `failed`) |

### Таблица `currency_rates`

Курсы валют, обновляются воркером `currency-sync` из API ЦБ РФ.

| Колонка      | Тип           | Описание                        |
|--------------|---------------|---------------------------------|
| `code`       | `VARCHAR(3)`  | Код валюты (`USD`, `EUR`)      |
| `rub_rate`   | `BIGINT`      | Курс к RUB в копейках          |
| `source_name`| `VARCHAR`     | Источник (`CBR`)               |
| `source_at`  | `TIMESTAMPTZ` | Дата курса от источника         |
| `created_at` | `TIMESTAMPTZ` | Время сохранения               |

`UNIQUE(code, source_name, source_at)` — защита от дублирования.

## Жизненный цикл платежа

### Статусы

| Статус      | Описание                                                |
| ----------- | ------------------------------------------------------- |
| `pending`   | Платёж создан, резерв в БД, ожидает инициализации       |
| `started`   | Провайдер принял запрос, платёж инициализирован         |
| `processed` | Precheckout одобрен (для Telegram), ожидает оплаты       |
| `completed` | Оплата подтверждена                                     |
| `failed`    | Отклонён провайдером или ошибка инициализации            |
| `cancelled` | Отменён (пользователем, по таймауту)                     |
| `refunded`  | Возврат средств                                         |
| `expired`   | Истёк срок действия                                      |

### Допустимые переходы

```
pending → started → processed → completed
                          ↓
                       failed
pending → failed
pending → cancelled
pending → expired
completed → refunded
cancelled → completed
```

## Идемпотентность

Каждый платёж создаётся с уникальным `idempotency_key`. При повторном запросе с тем же ключом:

- Если платёж в статусе `pending`/`started` и не истёк — возвращаем существующий URL
- Если платёж в статусе `started` и истёк (stale) — пересоздаём платёж у провайдера
- Если платёж в терминальном статусе — возвращаем ошибку
- Проверяется соответствие `user_id`, `amount`, `currency`, `description`, `provider_name`

## API (gRPC)

Сервис слушает на порту **44050**. Proto-определения находятся в `github.com/Krokozabra213/gargantua_common/gen/go/proto/payment/v1`.

### Методы

| Метод                | Описание                                      |
|----------------------|-----------------------------------------------|
| `TbankInit`          | Инициализация платежа через T-Bank            |
| `TbankWebhook`       | Обработка вебхука от T-Bank                   |
| `TGInit`             | Инициализация платежа через Telegram Stars    |
| `TGPreCheckout`      | Precheckout запрос от Telegram                |
| `TGSuccessful`       | Подтверждение успешной оплаты от Telegram     |
| `PaymentCancel`      | Отмена/возврат платежа                        |

## Outbox-воркер

Фоновый воркер `payments_outbox_worker` обрабатывает записи из `payments_outbox`:

1. Забирает батч записей в статусе `pending` с истёкшим `next_retry_at`
2. Отправляет protobuf-сообщение в Kafka топик `payments`
3. При успехе — помечает как `processed`
4. При ошибке — увеличивает счётчик попыток, применяет exponential backoff
5. После превышения `max_attempts` (10) — помечает как `failed`

## Воркер курсов валют

`currency-sync` (одноразовый запуск) запрашивает курсы USD и EUR с API ЦБ РФ и сохраняет в таблицу `currency_rates`. Курс USD используется при завершении Telegram-платежа для конвертации XTR в RUB.

## CI/CD

- **CI** ([`.github/workflows/ci.yaml`](.github/workflows/ci.yaml)): юнит-тесты → сборка → интеграционные тесты с test-postgres
- **Deploy** ([`.github/workflows/deploy.yaml`](.github/workflows/deploy.yaml)): сборка Docker-образов → push в GHCR → обновление манифестов в `GargantuaLabs/manifests-dev`

## Структура проекта

```
payment/
├── cmd/
│   ├── payment/main.go           # основной сервис
│   └── currency-sync/main.go     # воркер синхронизации курсов валют
├── configs/main.yml              # конфигурация
├── internal/
│   ├── app_error/                # кастомные ошибки
│   ├── config/                   # структуры конфигурации
│   ├── domain/                   # доменные модели
│   ├── kafka/producer/           # Kafka producer
│   ├── middleware/grpc/           # gRPC middleware
│   ├── providers/                # провайдеры (tbank, telegram)
│   ├── repository/postgres/      # слой БД
│   ├── server/grpc/              # gRPC сервер и хендлеры
│   ├── service/                  # бизнес-логика
│   ├── telemetry/                # метрики
│   └── workers/                  # фоновые воркеры
├── migrations/                   # SQL миграции
├── Dockerfile                    # продакшен образ
├── dev.Dockerfile                # dev образ с air (hot reload)
├── Migrate.Dockerfile            # образ для миграций
├── sync.Dockerfile               # образ для currency-sync
├── gowork.Dockerfile             # образ для разработки внутри go.work
├── docker-compose.yaml           # docker-compose (prod + test профили)
├── Makefile                      # команды запуска
├── .env.vault                    # зашифрованные переменные окружения
└── .github/workflows/            # CI/CD
```
