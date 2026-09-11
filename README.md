# finance-engine

Монолітний Go-бекенд для finance-rsmrtk: gRPC API, PostgreSQL, авторизація
через Sign in with Apple, фоновий синк курсів валют з НБУ. Один репозиторій,
один процес — без мікросервісів.

## Структура

```
api/v1/            .proto — джерело правди для API (сервіси + моделі)
api/v1/pb/          згенерований Go-код (make gen)
cmd/server/         точка входу: gRPC-сервер + фоновий синк курсів
internal/config/    конфігурація з env
internal/grpc/       сервер, інтерцептори (auth/logger/recovery), контролери
internal/service/    бізнес-логіка по доменах (auth, category, transaction, rate)
internal/repository/ доступ до БД поверх pkg/dbq
internal/ratesync/   фоновий тікер: тягне курси з НБУ, апсертить у БД
internal/db/queries/ .sql-запити для sqlc
pkg/dbq/            згенерований sqlc-код (типізовані запити)
pkg/jwt/            видача/перевірка JWT
pkg/appleauth/      перевірка Sign in with Apple identity token
pkg/pgutil/         конвертація pgtype <-> звичайні Go-типи
migrations/         SQL-міграції (goose)
deployments/        Dockerfile, Dockerfile.migrate, render.yaml, kind-cluster.yaml, k8s/
```

## Локальний запуск — kind (рекомендовано)

Живий кластер Kubernetes на своїй машині замість голого процесу — Postgres і
бекенд працюють як реальні Deployment'и, міграції — як Job, так само, як
виглядатиме прод. Потрібні `kind`, `kubectl`, `docker` (Colima теж підходить).

```bash
brew install kind kubectl docker

make kind-up        # створює кластер, мапить host :8443 -> сервіс
make kind-deploy     # білдить образи, завантажує в kind, ганяє міграції, деплоїть API
```

Далі `localhost:8443` — це вже gRPC API, що працює всередині кластера:

```bash
grpcurl -plaintext localhost:8443 list
```

Корисні команди:

```bash
make kind-status     # поди/сервіси/джоби в неймспейсі finance-engine
make kind-logs        # логи API-поду
make kind-redeploy    # перебілдити образ і перезапустити після зміни коду
make kind-down        # знести кластер повністю
```

Секрети (`JWT_SECRET`, `APPLE_BUNDLE_ID`, креденшли Postgres) лежать у
`deployments/k8s/*.yaml` як plaintext — це навмисно, оскільки це лише
локальний dev-кластер. Для прод-подібного k8s секрети мають йти через
Sealed Secrets / зовнішній secret-менеджер, не в git.

## Локальний запуск — голий процес (простіше, без Docker/k8s)

```bash
make tools          # protoc-gen-go, protoc-gen-go-grpc, sqlc, air, goose
brew install protobuf postgresql@17
brew services start postgresql@17

createdb finance_engine
export POSTGRES_DSN="postgres://localhost:5432/finance_engine?sslmode=disable"
make migrate-up

export JWT_SECRET="dev-secret"
export APPLE_BUNDLE_ID="com.rsmrtk.finance-rsmrtk"  # твій bundle id з Xcode

make live            # або: make run
```

## Зміна API (.proto)

1. Правиш файли в `api/v1/*.proto` або `api/v1/models/*.proto`
2. `make gen` — перегенерує Go-код і в `api/v1/pb`, і sqlc-запити

## Зміна схеми БД

1. `goose -dir migrations create <name> sql` — нова міграція
2. Пишеш SQL у `-- +goose Up` / `-- +goose Down`
3. Додаєш/міняєш запити в `internal/db/queries/*.sql`
4. `make gen` (перегенерує тільки sqlc, якщо proto не чіпав — просто `sqlc generate`)
5. У kind: `make kind-build && make kind-migrate` — міграції "запечені" в
   `finance-engine-migrate` образ (не ConfigMap), тож новий файл міграції
   потребує пересобрати образ

## Скидання БД (kind)

```bash
make kind-down && make kind-up && make kind-deploy
```

## Деплой на Render

1. Запуш репозиторій на GitHub
2. У Render: New → Blueprint → вкажи цей репозиторій, Render підхопить
   `deployments/render.yaml`
3. В UI Render постав секрет `APPLE_BUNDLE_ID` (позначено `sync: false`
   у render.yaml, тому не лежить у git)
4. Прогони міграції один раз проти prod БД:
   `POSTGRES_DSN=<render-connection-string> make migrate-up`

## Авторизація

Флоу навмисно прив'язаний до Apple-акаунта, а не до пристрою чи інсталяції
застосунку — це і є гарантія, що дані переживають перевстановлення:

```
iOS App --Sign in with Apple--> identity_token
iOS App --gRPC AuthService.SignInWithApple(identity_token)--> Backend
Backend: verify token signature (Apple JWKS) -> apple_sub
Backend: find-or-create user by apple_sub -> issue own JWT
iOS App: зберігає JWT у Keychain, шле в кожному виклику як
         `authorization: Bearer <jwt>` (gRPC metadata)
```

## Курси валют

`internal/ratesync` раз на 6 годин (і одразу при старті) тягне
`https://bank.gov.ua/NBU_Exchange/exchange?json` і апсертить у таблицю
`exchange_rates`. Окремий Render-сервіс для цього не потрібен — це
goroutine в тому самому процесі, що й gRPC-сервер.
