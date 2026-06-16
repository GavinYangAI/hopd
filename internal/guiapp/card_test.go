package guiapp

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"github.com/GavinYangAI/hopd/internal/ipc"
)

// diagramTexts flattens all canvas.Text strings in a forwardDiagram for asserting.
func diagramTexts(obj fyne.CanvasObject) []string {
	var out []string
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *canvas.Text:
			out = append(out, v.Text)
		case *fyne.Container:
			for _, c := range v.Objects {
				walk(c)
			}
		case *container.Scroll:
			walk(v.Content)
		}
	}
	walk(obj)
	return out
}

func containsText(texts []string, want string) bool {
	for _, s := range texts {
		if s == want {
			return true
		}
	}
	return false
}

func TestForwardDiagram_ViaHostNode(t *testing.T) {
	_ = test.NewApp()
	obj := forwardDiagram(ipc.TunnelStatus{
		Name: "pg", Local: "5432", Remote: "10.0.1.5:5432", ViaHost: "entryA",
	})
	texts := diagramTexts(obj)
	if !containsText(texts, "主机") {
		t.Fatalf("expected a 主机 caption node, got texts %v", texts)
	}
	if !containsText(texts, "entryA") {
		t.Fatalf("expected the host name entryA in the diagram, got %v", texts)
	}
}

func TestForwardDiagram_LegacyViaUnchanged(t *testing.T) {
	_ = test.NewApp()
	obj := forwardDiagram(ipc.TunnelStatus{
		Name: "t1", Local: "1", Remote: "h:2", Via: "bastion",
	})
	texts := diagramTexts(obj)
	if !containsText(texts, "中继") || !containsText(texts, "bastion") {
		t.Fatalf("legacy via diagram changed: %v", texts)
	}
}
