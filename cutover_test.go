package keyrotation

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeRotationAPI struct{ logs json.RawMessage }

func (f fakeRotationAPI) CreateMigrationKey(context.Context, string, string, string) (CreatedKey, error) {
	return CreatedKey{ID: "temporary-key"}, nil
}
func (f fakeRotationAPI) RotateMigrationKey(context.Context, string, int, string) (CreatedKey, error) {
	return CreatedKey{ID: "temporary-key", Key: "new-value-returned-once"}, nil
}
func (f fakeRotationAPI) SearchLogs(context.Context) (json.RawMessage, error) { return f.logs, nil }

func TestRunCutoverDecision(t *testing.T) {
	tests := []struct {
		name    string
		logs    string
		matches int
		ready   bool
	}{
		{name: "old deployment remains", logs: `[{"service":"etl-loader","key_marker":"key-v1"}]`, matches: 1, ready: false},
		{name: "all pipelines moved", logs: `[{"service":"warehouse-sync","key_marker":"key-v2"}]`, matches: 0, ready: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RunCutover(context.Background(), fakeRotationAPI{logs: json.RawMessage(tt.logs)}, CutoverInput{
				MigrationName: "pipeline-cutover", OldDeploymentMarker: "key-v1", GraceHours: 24, IdempotencyKey: "run-20260912",
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.OldMarkerMatches != tt.matches || got.ReadyForFinalCutover != tt.ready {
				t.Fatalf("got matches=%d ready=%v", got.OldMarkerMatches, got.ReadyForFinalCutover)
			}
		})
	}
}
