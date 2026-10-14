# Hiring instructions

Please refer to [these instructions](INSTRUCTIONS.md).

# Ads API

A small Go HTTP API around a single domain: *ads* (classified ads).

The API needs an up-and-running postgres database, which can be launched via the provided Docker Compose file.

## Requirements

- Go 1.26 or later
- Docker with the Compose plugin

## Launching the stack

```bash
docker compose up -d
```

This starts PostgreSQL with the schema and seed data already applied, the `ads` api, and a fake `users` api that the `ads` api calls to resolve the nickname of an ad owner.

The schema is applied when the database volume is created. An existing volume does not get the files added since then: run `docker compose down -v` to start over, or apply `ads/internal/cmd/ads-api/dao/postgres/migrations/003_publications_and_indexes.sql` by hand.

## Model

There are 2 entities in this sample:
- ad (title, price, etc...)
- user (id and nickname only)

## API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/ads` | yes | Create an ad, at most 5 per rolling 24 hours and per user |
| `GET` | `/ads/{id}` | no | Get a single ad, with the nickname of its owner |
| `GET` | `/ads` | no | List ads, newest first, filterable by `title`, `user_id`, `min_price`, `max_price`, paginated with `limit` and `offset` |
| `DELETE` | `/ads/{id}` | yes | Delete an ad, only its owner can |
| `POST` | `/ads/import` | yes | Bulk ad import, all or nothing |

The full contract is in [openapi.yaml](ads/internal/cmd/ads-api/specs/openapi.yaml)

```bash
# Health check
curl -s -v http://localhost:8080/health

# List ads (with optional filters)
curl -s 'http://localhost:8080/ads?title=bike&min_price=1000&limit=20'

# Get a single ad
curl -s http://localhost:8080/ads/1

# Create an ad (requires Authorization header)
curl -s -X POST http://localhost:8080/ads \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001' \
  -H 'Content-Type: application/json' \
  -d '{"title":"Mountain bike","price":12000,"photo_url":"https://example.com/photos/bike.jpg"}'

# Delete an ad (only its owner can)
curl -s -X DELETE http://localhost:8080/ads/1 \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001'
```

## Authentication

To pass authentication information to a call, simply add the `Authorization` header with the user ID in plain text.

It will be extracted by the service.

```
Authorization: 00000000-0000-0000-0000-000000000001
```

## Behaviors worth knowing

- Prices are integers in euro cents. The field and the filters were named `price_cents`, `min_price_cents` and `max_price_cents`, they are now `price`, `min_price` and `max_price`
- The former names still work, so that web and mobile clients that cannot be updated at once keep working: requests accept either name (and reject two different values for the same field), responses carry both `price` and `price_cents` with the same value. Remove the former names once no client uses them anymore
- `GET /ads` returns at most 100 ads per call (50 by default), newest first, use `limit` and `offset` to page through them
- `GET /ads/{id}` adds the `username` of the owner. The user service answers are cached in memory (5 minutes for a known user, 1 minute for an unknown one, 5 seconds for a failure) and concurrent lookups of one user share a single call. When the user service is down or does not know the user the ad is still returned, without `username`
- `POST /ads` answers `429` with a `Retry-After` header once a user published 5 ads over the last 24 hours. Publications are kept in their own table, so deleting an ad does not give quota back and ads created by the bulk import are not counted. Concurrent requests of one user are serialized with a PostgreSQL advisory lock, so they cannot get past the quota together
- `POST /ads/import` runs in a single transaction: when it fails nothing was created and it can be run again without duplicating ads. It holds at most 1000 ads, and an invalid ad rejects the import with a message naming it (for example `ads[3]`). The quota does not apply
- `DELETE /ads/{id}` answers `403` when the ad belongs to someone else and `404` when it does not exist

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_ADDR` | `:8080` | Address the api listens on |
| `DATABASE_URL` | `postgres://ads:ads@localhost:5432/ads?sslmode=disable` | PostgreSQL connection string |
| `IDENTITY_URL` | `http://localhost:8081` | Base URL of the user service |
| `IDENTITY_TIMEOUT` | `2s` | Bound on every call to the user service |

## Tests

To launch all the tests:

```shell
go test ./...
```
