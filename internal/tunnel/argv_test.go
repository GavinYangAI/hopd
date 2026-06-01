package tunnel

import (
	"reflect"
	"testing"

	"github.com/GavinYangAI/hopd/internal/config"
)

func TestBuildArgs_ViaAlias(t *testing.T) {
	tn := config.Tunnel{
		Name:   "prod-db",
		Local:  "5432",
		Remote: "10.0.1.5:5432",
		Via:    "prod-bastion",
		SSHOptions: map[string]string{
			"ExitOnForwardFailure": "yes",
		},
	}
	got := BuildArgs(tn)
	want := []string{
		"-N", "-T",
		"-o", "ExitOnForwardFailure=yes",
		"-L", "127.0.0.1:5432:10.0.1.5:5432",
		"prod-bastion",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildArgs() =\n  %v\nwant\n  %v", got, want)
	}
}

func TestBuildArgs_InlineJump_NoVia(t *testing.T) {
	tn := config.Tunnel{
		Name:   "stg-web",
		Local:  "127.0.0.1:8080",
		Remote: "127.0.0.1:80",
		Jump:   []string{"user@jump1", "user@jump2"},
	}
	got := BuildArgs(tn)
	want := []string{
		"-N", "-T",
		"-J", "user@jump1,user@jump2",
		"-L", "127.0.0.1:8080:127.0.0.1:80",
		"127.0.0.1", // target host derived from remote when no via
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildArgs() =\n  %v\nwant\n  %v", got, want)
	}
}

func TestBuildArgs_JumpPlusVia_SortedOptions(t *testing.T) {
	tn := config.Tunnel{
		Name:   "x",
		Local:  "2000",
		Remote: "svc:9000",
		Via:    "backend",
		Jump:   []string{"j1"},
		SSHOptions: map[string]string{
			"ServerAliveInterval": "15",
			"ConnectTimeout":      "5",
		},
	}
	got := BuildArgs(tn)
	want := []string{
		"-N", "-T",
		"-o", "ConnectTimeout=5", // options emitted in sorted key order
		"-o", "ServerAliveInterval=15",
		"-J", "j1",
		"-L", "127.0.0.1:2000:svc:9000",
		"backend",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildArgs() =\n  %v\nwant\n  %v", got, want)
	}
}
