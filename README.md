# habr-crawler-go

A simple web crawler + HTTP API:
- Crawl Habr posts for the current day and store them in PostgreSQL.
- API:
  - `GET /api/users?start=<unix>&end=<unix>`
  - `GET /api/posts?user=<name>&start=<unix>&end=<unix>`
  - `GET /api/idf?word=<word>&start=<unix>&end=<unix>`
  - `POST /api/update` (crawl today)

## Prereqs
- Go 1.23+
- Docker Desktop (for local PostgreSQL)

## Quick start (Docker Compose)
```bash
cp .env.example .env
make start-all
```

API will be at `http://localhost:8100`

### Try it
Crawl today's posts:
```bash
curl -X POST http://localhost:8100/api/update | jq
```

List authors for the last 24h:
```bash
curl "http://localhost:8100/api/users" | jq
```

Get posts by an author:
```bash
curl "http://localhost:8100/api/posts?user=someauthor" | jq
```

Compute IDF:
```bash
curl "http://localhost:8100/api/idf?word=docker" | jq
```

## Local run (without containerizing the API)
1) Start Postgres:
```bash
cp .env.example .env
make db-up
```

2) Pull deps:
```bash
make tidy
```

3) Run API:
```bash
make run-api
```

4) Or run crawler once:
```bash
make crawl
```
