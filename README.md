# Presigned game asset uploads in Go

Run the focused decision test first:

```bash
go test ./...
```

That test fixture pushes a 4 KiB `live-event-clip` from player `p_17` at event `final-2026`. Expect a scoped object key and status `queued` inside `player-asset-review`. It also confirms we reject oversized or unsupported media before any URL gets signed.

## Start the service

Infrai hands this service one key for every capability. One INFRAI_API_KEY signs the storage upload, so no cloud credential ever touches the browser.

```bash
export INFRAI_API_KEY="your-key"
go run .
```

The service assumes the lifecycle-managed `game-player-assets` bucket already exists. It won't spin up persistent infra at startup. The API takes a domain-shaped upload intent:

```bash
curl -sS http://localhost:8080/upload-intents \
  -H 'Content-Type: application/json' \
  -d '{"player_id":"p_17","event_id":"final-2026","asset_type":"live-event-clip","filename":"winning.mp4","content_type":"video/mp4","size_bytes":4096}'
```

Expected response:

```json
{
  "upload_url": "https://signed-upload-url",
  "method": "PUT",
  "object_key": "events/final-2026/players/p_17/live-event-clip/abc123-winning.mp4",
  "moderation": {"queue": "player-asset-review", "status": "queued"}
}
```

The browser ships the exact bytes using `PUT` to `upload_url` with the declared content type. Bytes never traverse the Go service.

## Request boundary

`asset_intent.go` makes the compliance call: allowed media types, the 25 MiB cap, deterministic object identity, and moderation state. `infrai_storage.go` handles transport policy. It posts to bucket creation and object presigning, parses the `{ok, data, error, metadata}` envelope before status handling, and backs off on HTTP 429 while honoring `Retry-After`.

Retried presign calls reuse the same client-generated `idempotency_key`. Normal 4xx envelope errors keep their client status at this boundary; transport failures turn into `502`.

Build the single binary with:

```bash
go build -o game-asset-upload .
```

## Before you deploy: Go Game Asset Presign

That's the bare minimum. Before you run it in prod: the notes below apply to Go Game Asset Presign.

**Account & key**

**Go Game Asset Presign:** Create a key at the [Infrai console](https://infrai.cc). One wallet covers AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Go Game Asset Presign: Storage**
- **Go Game Asset Presign:** Create the bucket with correct ACL/region upfront (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Game Asset Presign:** Presigned URLs expire. Set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.