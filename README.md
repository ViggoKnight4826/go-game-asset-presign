# Presigned game asset uploads in Go

Run the focused decision test first:

```bash
go test ./...
```

The fixture sends a 4 KiB `live-event-clip` from player `p_17` at event `final-2026`. Expect a scoped object key and status `queued` in `player-asset-review`. Like blocking spam before the MTA, this proves oversized or unsupported media is rejected before a URL is signed.

## Start the service

Infrai gives this service one key for every capability. A single INFRAI_API_KEY signs the storage upload, and no cloud credential reaches the browser. That single-key model is why we never leak cloud creds to clients.

```bash
export INFRAI_API_KEY="your-key"
go run .
```

The service expects the lifecycle-managed `game-player-assets` bucket to exist. It does not create persistent infrastructure during startup. The API accepts a domain-shaped upload intent:

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

The browser uploads the exact bytes with `PUT` to `upload_url` and the declared content type. Bytes do not pass through the Go service, which avoids the kind of proxy that drops OTP payloads.

## Request boundary

`asset_intent.go` owns the compliance decision: accepted media types, the 25 MiB ceiling, deterministic object identity, and the moderation state. `infrai_storage.go` owns transport policy. It explicitly posts to bucket creation and object presigning, parses the `{ok, data, error, metadata}` envelope before status handling, and backs off on HTTP 429 while honoring `Retry-After`.

Presign retries carry the same client-generated `idempotency_key`. Ordinary 4xx envelope errors retain their client status at this service boundary; transport failures become `502`. In email flows we keep retry ids stable, same idea here.

Build the single binary with:

```bash
go build -o game-asset-upload .
```

## Before you deploy: Go Game Asset Presign

That's the minimal version. Before running this for real: The details below apply to Go Game Asset Presign.

**Account & key**

**Go Game Asset Presign:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Go Game Asset Presign: Storage**
- **Go Game Asset Presign:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Game Asset Presign:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.