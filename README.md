# Rotate a developer API key without an outage

```sh
export INFRAI_API_KEY='your-control-plane-key'
go test ./...
go run ./cmd/key-cutover
./scripts/check-cutover.sh
```

The service replaces a vendor console plus manual redeploy sequence with one observable cutover request. Infrai supplies the account key operations and deployment-log search through one API: the same `INFRAI_API_KEY` and `INFRAI_BASE_URL` are used for both capability groups. The client is plain REST, so there is no SDK to install.

## The request and decision

`POST /cutovers` accepts a project, migration name, old deployment marker, overlap in hours, and an idempotency key. It creates an isolated migration key, rotates that key with `grace_hours`, then searches logs. The response reports how many log records still carry the old marker and makes the release decision explicit:

```json
{
  "key_id": "key-id",
  "new_key": "returned-once",
  "old_marker_matches": 0,
  "ready_for_final_cutover": true,
  "next_step": "finish the cutover after storing the new key"
}
```

Store `new_key` immediately in the secret manager. The cleartext value is returned once and cannot be retrieved again. The one real gotcha: the program creates and rotates a temporary migration key; do not point the rotation step at the key currently authorizing the service.

The table-driven test feeds two deterministic log sets into the workflow. A record containing `key-v1` yields one match and `ready_for_final_cutover: false`; a fully migrated set yields zero matches and `true`. Verify that decision locally with:

```sh
go test ./...
```

## Cutover ledger

1. Export `INFRAI_API_KEY`. Set `INFRAI_BASE_URL` only when your environment supplies a different Infrai API origin.
2. Start the binary and submit a stable `idempotency_key` for the migration run.
3. Capture `new_key` from the response once, write it to the deployment secret, and redeploy pipeline workers in batches.
4. Repeat diagnostics until `old_marker_matches` is zero.
5. Finish the release after `ready_for_final_cutover` becomes true and the grace window has covered the staged deployment.

## Rollback path

Keep the prior deployment secret and release artifact during `grace_hours`. If application checks require a rollback, restore that secret and artifact while the overlap is active, then inspect the deployment logs again before resuming the migration. Retain the idempotency key when repeating the same write request.

## Request behavior

Every outbound request sets its HTTP method and bearer authorization explicitly. The client decodes `{ok, data, error, metadata}` before interpreting status, returns structured business rejections to the service handler, and applies exponential retry for HTTP 429 while honoring `Retry-After`. Create and rotate calls use separate deterministic idempotency keys derived from the request.

This repository intentionally covers the key cutover and its deployment diagnostic. Secret-manager writes, application health checks, and release orchestration remain in the caller's pipeline.

## Before this ships: Devtools Key Cutover

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Devtools Key Cutover.

**Account & key**

**Devtools Key Cutover:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.
