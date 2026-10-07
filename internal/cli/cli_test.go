package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devsync/dev-sync/internal/logging"
)

func TestCLI_InitAddListDoctor(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	repDir := filepath.Join(tmpDir, "replica1")

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repDir, 0755); err != nil {
		t.Fatal(err)
	}

	defer func() {
		logging.Close()
		_ = os.Chdir(origWd)
	}()

	// 1. Run init in non-interactive mode
	initCode := Run([]string{"init", "--source", sourceDir, "--non-interactive", "--yes"})
	if initCode != ExitSuccess {
		t.Fatalf("init returned non-zero code: %d", initCode)
	}

	_ = os.Chdir(sourceDir)

	// 2. Run add
	addCode := Run([]string{"add", "--id", "my-replica", "--path", repDir})
	if addCode != ExitSuccess {
		t.Fatalf("add returned non-zero code: %d", addCode)
	}

	// 3. Run list
	listCode := Run([]string{"list"})
	if listCode != ExitSuccess {
		t.Fatalf("list returned non-zero code: %d", listCode)
	}

	// 4. Run doctor
	doctorCode := Run([]string{"doctor"})
	if doctorCode != ExitSuccess {
		t.Fatalf("doctor returned non-zero code: %d", doctorCode)
	}

	// 5. Run status
	statusCode := Run([]string{"status"})
	if statusCode != ExitSuccess {
		t.Fatalf("status returned non-zero code: %d", statusCode)
	}

	// 6. Run sync dry-run
	syncDryCode := Run([]string{"sync", "--dry-run"})
	if syncDryCode != ExitSuccess {
		t.Fatalf("sync dry-run returned non-zero code: %d", syncDryCode)
	}

	// 7. Run real sync
	syncCode := Run([]string{"sync"})
	if syncCode != ExitSuccess {
		t.Fatalf("sync returned non-zero code: %d", syncCode)
	}

	// 8. Run verify
	verifyCode := Run([]string{"verify"})
	if verifyCode != ExitSuccess {
		t.Fatalf("verify returned non-zero code: %d", verifyCode)
	}

	// 9. Run version
	versionCode := Run([]string{"version"})
	if versionCode != ExitSuccess {
		t.Fatalf("version returned non-zero code: %d", versionCode)
	}
}
