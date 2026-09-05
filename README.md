# Thousand Sunny

Personal Netflix-style film streaming with a One Piece / Thousand Sunny theme. No authentication — local use only.

## Stack

- **Traefik** — reverse proxy on port 80 (`v3.6`, Docker API compatible with modern daemons)
- **Vue 3** frontend — library, series, search, player, admin upload
- **Go** API — Postgres metadata + file upload/stream
- **Postgres 16** — `series` + `animations` tables

## Quick start

```bash
docker compose up --build
```

Open [http://localhost](http://localhost).

- **Home** — series cards + standalone films (search both)
- **Series** — seasons / parts list for a franchise or show
- **Watch** — HTML5 player with range seeking
- **Galley-La (`/admin`)** — create series, upload films with season/episode/sort

## Grouping films

1. Create a **series** (e.g. Dune, Bleach) with kind `franchise`, `anime`, or `tv`.
2. Upload each film/episode and assign it to that series.
3. Use **sort / part** for movie trilogies (Part 1, Part 2).
4. Use **season + episode** for TV/anime.

## Environment (backend)

| Variable | Default (Compose) | Purpose |
|----------|-------------------|---------|
| `DATABASE_URL` | `postgres://sunny:sunny@postgres:5432/thousand_sunny?sslmode=disable` | Postgres connection |
| `MEDIA_ROOT` | `/data` | Video + poster storage |
| `PORT` | `8080` | API listen port |

Media files live in the Docker volume `media_data`. Prefer **H.264 MP4** for browser playback.

## Local frontend dev (optional)

```bash
# Start API + DB with Compose, or run the Go binary against local Postgres
cd frontend && npm install && npm run dev
```

Vite proxies `/api` to `http://localhost:8080`.

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/library?q=` | Home grid: series + standalone films |
| `GET` | `/api/series?q=` | List series |
| `GET` | `/api/series/{id}` | Series + ordered entries |
| `GET` | `/api/series/{id}/poster` | Series cover (or first entry) |
| `POST` | `/api/series` | Multipart: `name`, `description`, `kind`, optional `poster` |
| `DELETE` | `/api/series/{id}` | Delete series + all entries/files |
| `GET` | `/api/animations?q=` | List / search all films |
| `GET` | `/api/animations/{id}` | Metadata |
| `GET` | `/api/animations/{id}/poster` | Poster image |
| `GET` | `/api/animations/{id}/stream` | Video (byte-range) |
| `POST` | `/api/animations` | Multipart: `name`, `description`, `video`, `poster`, optional `series_id`, `season`, `episode`, `sort_order` |
| `DELETE` | `/api/animations/{id}` | Delete row + files |
| `GET` | `/api/health` | Health check |

## Security note

There is no auth. Do not expose this stack to the public internet without Traefik basic-auth, a VPN, or similar in front of at least `/admin`.
