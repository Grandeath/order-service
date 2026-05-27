# order-service

Go microservice odpowiedzialny za zarządzanie zamówieniami w architekturze:

```
Vue.js Frontend → API Gateway → Order Service → (Product, Payment, Kafka, Postgres, Analytics)
```

Stos: Go (`net/http` + chi), PostgreSQL (`pgx/v5`), Kafka (`franz-go`),
Prometheus. Repozytorium in-memory zostawione jest jako alternatywna
implementacja `Repository` (używana w testach handlerów).

## Odpowiedzialności

- przyjmowanie żądań utworzenia zamówienia,
- walidacja danych wejściowych,
- zapis zamówień (in-memory / pluggable repo),
- zarządzanie cyklem życia zamówienia (state machine),
- publikowanie zdarzeń domenowych (interfejs `events.Publisher`),
- udostępnianie REST API dla klienta i administratora,
- idempotencja tworzenia zamówień (`Idempotency-Key`).

## Cykl życia zamówienia

```
NEW → VALIDATED → PAYMENT_PENDING → PAID → PROCESSING → SHIPPED
            ↓             ↓           ↓          ↓
        CANCELLED / FAILED (z dowolnego stanu nieterminalnego)
```

## Uruchomienie

Wymagane usługi (przykładowo `docker run`):

```bash
docker run -d --name pg \
  -e POSTGRES_USER=order -e POSTGRES_PASSWORD=order -e POSTGRES_DB=orders \
  -p 5432:5432 postgres:16
```

Kafka jest opcjonalna — przy `KAFKA_ENABLED=false` events lecą do `/dev/null`
po stronie producera (`producer.EventNotifier.enabled = false`).

```bash
CONFIG_NAME=.env go run .
```

- API: `http://localhost:8080`
- Technical (ping, metrics, pprof, version): `http://localhost:9090`

Schemat bazy (`orders`, `idempotency_keys`) aplikuje się automatycznie przy
starcie przez `db.Migrate` (`CREATE TABLE IF NOT EXISTS`).

## Uwierzytelnianie (AWS Cognito)

Logowanie odbywa się przez Cognito User Pools — serwis weryfikuje wystawiane
przez nie tokeny JWT (`lestrrat-go/jwx`). Włączenie:

```env
COGNITO_ENABLED=true
COGNITO_REGION=eu-central-1
COGNITO_USER_POOL_ID=eu-central-1_xxxxxxxx
COGNITO_APP_CLIENT_ID=xxxxxxxxxxxxxxxxxxxxxxxxxx   # opcjonalne
```

Sprawdzane są:

- podpis (klucze pobierane z `https://cognito-idp.<region>.amazonaws.com/<userPoolId>/.well-known/jwks.json` i cache'owane z refreshem 15 min – 24 h),
- `iss` (oczekiwany dokładnie issuer Cognito user pool),
- `exp` / `iat` (z 30 s zegarowej tolerancji),
- `token_use` — `access` albo `id`,
- `aud` (dla id token) lub `client_id` (dla access token), gdy ustawione `COGNITO_APP_CLIENT_ID`.

Klient wysyła nagłówek `Authorization: Bearer <jwt>`. Brak / niepoprawny token
to `401 Unauthorized` z `WWW-Authenticate: Bearer`. Token jest dostępny w
handlerach przez `auth.TokenFromContext(ctx)`, a `sub` przez `auth.SubjectFromContext(ctx)`.

## Endpointy

| Metoda | Ścieżka                      | Opis                                        |
|--------|------------------------------|---------------------------------------------|
| POST   | `/api/v1/orders`             | Utworzenie zamówienia                       |
| GET    | `/api/v1/orders`             | Lista zamówień (`?customerId&limit&offset`) |
| GET    | `/api/v1/orders/{id}`        | Szczegóły zamówienia                        |
| PATCH  | `/api/v1/orders/{id}/status` | Zmiana statusu (state machine)              |
| POST   | `/api/v1/orders/{id}/cancel` | Anulowanie zamówienia                       |
| DELETE | `/api/v1/orders/{id}`        | Usunięcie (admin)                           |

### Identyfikator klienta

`customerId` **nie** jest częścią ciała żądania — ustala go warstwa auth w jednym
miejscu (middleware):

- `COGNITO_ENABLED=true` → z claimu `sub` zweryfikowanego tokenu JWT,
- `COGNITO_ENABLED=false` (dev) → z parametru `?customerId=` (lub nagłówka `X-Customer-Id`).

Handlery czytają go wyłącznie z kontekstu (`auth.CustomerIDFromContext`).

### Przykład: utworzenie zamówienia (tryb dev, bez Cognito)

```bash
curl -X POST 'http://localhost:8080/api/v1/orders?customerId=cust-1' \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: 8a3f-...' \
  -d '{
    "currency": "PLN",
    "items": [
      {"productId":"p1","productName":"Książka","quantity":2,"unitPrice":"39.90"}
    ],
    "deliveryAddress": {
      "street": "Marszałkowska 1",
      "city": "Warszawa",
      "postalCode": "00-001",
      "country": "PL"
    }
  }'
```

Z włączonym Cognito `customerId` z query jest ignorowany — liczy się `sub` z
`Authorization: Bearer <jwt>`.

### Przykład: zmiana statusu

```bash
curl -X PATCH http://localhost:8080/api/v1/orders/<id>/status \
  -H 'Content-Type: application/json' \
  -d '{"status":"VALIDATED"}'
```

## Struktura

```
internal/
  config/                konfiguracja env + logger
  db/                    pool pgx + migracja schematu
  metrics/               Prometheus
  producer/              klient Kafka (franz-go) z metrykami kprom
  server/                worker HTTP (chi) + technical endpoints
  utils/                 helpery (WriteJSON, AppVersion)
  order/
    domain/              Order, OrderItem, Address, OrderStatus, state machine, błędy
    events/              Publisher: NoopPublisher (logi) + KafkaPublisher (producer)
    repository/          Repository: InMemory (testy) + Postgres (pgx)
    service/             warstwa aplikacyjna (Create/Get/List/Advance/Cancel/Delete)
    api/                 DTO, handlery HTTP, middleware
```

## Testy

```bash
go test ./...
```

Pokrycie: walidacja domeny, state machine, idempotencja, mapowanie błędów na
kody HTTP, scenariusze handlerów (happy path + edge cases).
