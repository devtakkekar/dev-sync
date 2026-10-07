package syncengine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devsync/dev-sync/internal/config"
	"github.com/devsync/dev-sync/internal/drift"
	"github.com/devsync/dev-sync/internal/verify"
)

func TestSyncEngine_EndToEndLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	replicaDir := filepath.Join(tmpDir, "replica1")

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. Create source structure
	file1 := filepath.Join(sourceDir, "file1.txt")
	file2 := filepath.Join(sourceDir, "sub", "file2.txt")
	if err := os.MkdirAll(filepath.Dir(file2), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file1, []byte("Hello World"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("Nested Content"), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Setup project config
	cfg := config.DefaultConfig(sourceDir)
	cfg.Replicas = append(cfg.Replicas, config.ReplicaConfig{
		ID:      "rep1",
		Path:    replicaDir,
		Enabled: true,
	})
	if err := config.SaveConfig(sourceDir, cfg); err != nil {
		t.Fatal(err)
	}

	engine, err := NewSyncEngine(sourceDir)
	if err != nil {
		t.Fatalf("failed to create sync engine: %v", err)
	}

	ctx := context.Background()

	// 3. Dry-run sync
	dryResults, err := engine.Sync(ctx, EngineSyncOptions{
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if len(dryResults) != 1 || dryResults[0].Created != 2 {
		t.Fatalf("expected 2 planned creates in dry run, got %+v", dryResults[0])
	}

	// Ensure dry run did not create replica
	if _, err := os.Stat(replicaDir); !os.IsNotExist(err) {
		t.Fatalf("dry run should not have created replica on disk")
	}

	// 4. Real sync
	results, err := engine.Sync(ctx, EngineSyncOptions{})
	if err != nil {
		t.Fatalf("real sync failed: %v", err)
	}
	if len(results) != 1 || results[0].Created != 2 {
		t.Fatalf("expected 2 created files, got %+v", results[0])
	}

	// Verify files in replica
	repFile1 := filepath.Join(replicaDir, "file1.txt")
	repFile2 := filepath.Join(replicaDir, "sub", "file2.txt")

	data1, err := os.ReadFile(repFile1)
	if err != nil || string(data1) != "Hello World" {
		t.Fatalf("repFile1 mismatch: %s, err: %v", string(data1), err)
	}

	data2, err := os.ReadFile(repFile2)
	if err != nil || string(data2) != "Nested Content" {
		t.Fatalf("repFile2 mismatch: %s, err: %v", string(data2), err)
	}

	// 5. Verify status using verify.VerifyReplica
	verRes, err := verify.VerifyReplica("rep1", sourceDir, replicaDir, engine.Matcher, "skip")
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if !verRes.InSync || verRes.MatchedFiles != 2 {
		t.Fatalf("expected 2 matched in-sync files, got %+v", verRes)
	}

	// 6. Test File Modification Sync
	if err := os.WriteFile(file1, []byte("Updated Content for File 1"), 0644); err != nil {
		t.Fatal(err)
	}

	modResults, err := engine.Sync(ctx, EngineSyncOptions{})
	if err != nil {
		t.Fatalf("mod sync failed: %v", err)
	}
	if modResults[0].Modified != 1 {
		t.Fatalf("expected 1 modified file, got %+v", modResults[0])
	}

	modData1, _ := os.ReadFile(repFile1)
	if string(modData1) != "Updated Content for File 1" {
		t.Fatalf("replica file1 was not updated: %s", string(modData1))
	}

	// 7. Test Extraneous File Deletion Sync
	if err := os.Remove(file2); err != nil {
		t.Fatal(err)
	}

	delResults, err := engine.Sync(ctx, EngineSyncOptions{})
	if err != nil {
		t.Fatalf("del sync failed: %v", err)
	}
	if delResults[0].Deleted != 1 {
		t.Fatalf("expected 1 deleted file, got %+v", delResults[0])
	}

	if _, err := os.Stat(repFile2); !os.IsNotExist(err) {
		t.Fatalf("file2 should have been deleted from replica")
	}

	// 8. Test Drift Detection & Repair
	extraReplicaFile := filepath.Join(replicaDir, "rogue.txt")
	if err := os.WriteFile(extraReplicaFile, []byte("I was created on replica!"), 0644); err != nil {
		t.Fatal(err)
	}

	driftReport, err := drift.DetectDrift("rep1", sourceDir, replicaDir, engine.Matcher, "skip")
	if err != nil {
		t.Fatalf("detect drift failed: %v", err)
	}
	if !driftReport.HasDrift || len(driftReport.Extra) != 1 || driftReport.Extra[0] != "rogue.txt" {
		t.Fatalf("expected drift report with rogue.txt extra, got %+v", driftReport)
	}

	// Auto-repair drift
	repairRes, err := drift.RepairDrift(ctx, sourceDir, "rep1", replicaDir, engine.Matcher, "skip", 64)
	if err != nil {
		t.Fatalf("repair drift failed: %v", err)
	}
	if repairRes.Deleted != 1 {
		t.Fatalf("expected 1 deleted rogue file during repair, got %+v", repairRes)
	}
	if _, err := os.Stat(extraReplicaFile); !os.IsNotExist(err) {
		t.Fatalf("rogue file should have been removed during drift repair")
	}
}
