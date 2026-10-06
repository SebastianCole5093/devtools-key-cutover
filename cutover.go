package keyrotation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type RotationAPI interface {
	CreateMigrationKey(context.Context, string, string, string) (CreatedKey, error)
	RotateMigrationKey(context.Context, string, int, string) (CreatedKey, error)
	SearchLogs(context.Context) (json.RawMessage, error)
}

type CutoverInput struct {
	ProjectID           string `json:"project_id"`
	MigrationName       string `json:"migration_name"`
	OldDeploymentMarker string `json:"old_deployment_marker"`
	GraceHours          int    `json:"grace_hours"`
	IdempotencyKey      string `json:"idempotency_key"`
}

type CutoverResult struct {
	KeyID                string `json:"key_id"`
	NewKey               string `json:"new_key"`
	OldMarkerMatches     int    `json:"old_marker_matches"`
	ReadyForFinalCutover bool   `json:"ready_for_final_cutover"`
	NextStep             string `json:"next_step"`
}

func RunCutover(ctx context.Context, api RotationAPI, in CutoverInput) (CutoverResult, error) {
	if in.GraceHours <= 0 || in.IdempotencyKey == "" || in.OldDeploymentMarker == "" {
		return CutoverResult{}, errors.New("grace_hours, idempotency_key, and old_deployment_marker are required")
	}
	created, err := api.CreateMigrationKey(ctx, in.ProjectID, in.MigrationName, in.IdempotencyKey+"-create")
	if err != nil {
		return CutoverResult{}, fmt.Errorf("create migration key: %w", err)
	}
	rotated, err := api.RotateMigrationKey(ctx, created.ID, in.GraceHours, in.IdempotencyKey+"-rotate")
	if err != nil {
		return CutoverResult{}, fmt.Errorf("rotate migration key: %w", err)
	}
	logs, err := api.SearchLogs(ctx)
	if err != nil {
		return CutoverResult{}, fmt.Errorf("search deployment logs: %w", err)
	}
	matches := strings.Count(string(logs), in.OldDeploymentMarker)
	result := CutoverResult{KeyID: rotated.ID, NewKey: rotated.Key, OldMarkerMatches: matches}
	result.ReadyForFinalCutover = matches == 0
	if result.ReadyForFinalCutover {
		result.NextStep = "finish the cutover after storing the new key"
	} else {
		result.NextStep = "redeploy the remaining old-key consumers, then run diagnostics again"
	}
	return result, nil
}
