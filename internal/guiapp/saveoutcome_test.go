package guiapp

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/GavinYangAI/hopd/internal/gui"
)

func TestRejectionMessage_SurfacesDaemonReason(t *testing.T) {
	// The whole point of the fix: a saved-but-rejected config must be reported
	// with the daemon's real reason, not the misleading "daemon 未运行".
	err := fmt.Errorf("%w: %w", gui.ErrReloadAfterSave, &gui.DaemonRejected{Reason: `tunnel "sa-data": must set via or jump`})
	msg, ok := rejectionMessage(err)
	if !ok {
		t.Fatal("a daemon rejection must be recognised")
	}
	if !strings.Contains(msg, `must set via or jump`) {
		t.Fatalf("message must carry the daemon's reason, got %q", msg)
	}
	if strings.Contains(msg, "未运行") {
		t.Fatalf("a rejection must not be reported as not-running, got %q", msg)
	}
}

func TestRejectionMessage_IgnoresUnreachable(t *testing.T) {
	err := fmt.Errorf("%w: %w", gui.ErrReloadAfterSave, fmt.Errorf("%w: refused", gui.ErrDaemonUnreachable))
	if _, ok := rejectionMessage(err); ok {
		t.Fatal("a mere unreachable daemon is not a rejection")
	}
}

func TestRejectionMessage_IgnoresPlainSaveFailure(t *testing.T) {
	if _, ok := rejectionMessage(errors.New("disk full")); ok {
		t.Fatal("a generic save failure is not a rejection")
	}
}
