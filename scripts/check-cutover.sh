#!/bin/sh
set -eu

curl --fail-with-body --request POST "${SERVICE_URL:-http://localhost:8080}/cutovers" \
  --header 'Content-Type: application/json' \
  --data '{"project_id":"etl-platform","migration_name":"pipeline-cutover","old_deployment_marker":"key-v1","grace_hours":24,"idempotency_key":"migration-run-001"}'
