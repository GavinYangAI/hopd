package tunnel

import (
	"reflect"
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
)

func TestControlOptions(t *testing.T) {
	co := ControlOptions("/cm", "no")
	if co["ControlMaster"] != "auto" {
		t.Fatalf("ControlMaster = %q", co["ControlMaster"])
	}
	if co["ControlPersist"] != "no" {
		t.Fatalf("ControlPersist = %q", co["ControlPersist"])
	}
	if co["ControlPath"] != "/cm/%C" {
		t.Fatalf("ControlPath = %q", co["ControlPath"])
	}
	if got := ControlOptions("/cm", "300")["ControlPersist"]; got != "300" {
		t.Fatalf("persist arg should pass through, got %q", got)
	}
}

func TestAuthArgs(t *testing.T) {
	tn := config.Tunnel{
		Name:       "x",
		Remote:     "svc:9000",
		Via:        "backend",
		Jump:       []string{"j1"},
		SSHOptions: map[string]string{},
	}
	got := AuthArgs(tn, "/cm")
	want := []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=/cm/%C",
		"-o", "ControlPersist=300",
		"-J", "j1",
		"backend", "true",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AuthArgs() =\n  %v\nwant\n  %v", got, want)
	}
}
