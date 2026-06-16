package gui

import "testing"

func TestCheckHost_OK(t *testing.T) {
	f := HostForm{Name: "entryA", Host: "198.51.100.7", Port: "65522", User: "u", Jump: "bastionB"}
	errs := CheckHost(f, []string{"other"}, []string{"bastionB", "other"})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestCheckHost_NameRules(t *testing.T) {
	base := HostForm{Host: "h"}
	if CheckHost(base, nil, nil)["name"] == "" {
		t.Fatal("empty name should error")
	}
	f := base
	f.Name = "has space"
	if CheckHost(f, nil, nil)["name"] == "" {
		t.Fatal("name with space should error")
	}
	f = base
	f.Name = "dup"
	if CheckHost(f, []string{"dup"}, nil)["name"] == "" {
		t.Fatal("duplicate name should error")
	}
}

func TestCheckHost_HostRequired(t *testing.T) {
	if CheckHost(HostForm{Name: "a"}, nil, nil)["host"] == "" {
		t.Fatal("empty host should error")
	}
}

func TestCheckHost_Port(t *testing.T) {
	ok := HostForm{Name: "a", Host: "h"}
	if CheckHost(ok, nil, nil)["port"] != "" {
		t.Fatal("empty port should be allowed (defaults to 22)")
	}
	bad := ok
	bad.Port = "70000"
	if CheckHost(bad, nil, nil)["port"] == "" {
		t.Fatal("out-of-range port should error")
	}
	bad = ok
	bad.Port = "-p22"
	if CheckHost(bad, nil, nil)["port"] == "" {
		t.Fatal("port with -p should error")
	}
}

func TestCheckHost_Jump(t *testing.T) {
	base := HostForm{Name: "a", Host: "h"}
	ok := base
	ok.Jump = ""
	if CheckHost(ok, nil, nil)["jump"] != "" {
		t.Fatal("empty jump should be allowed")
	}
	unknown := base
	unknown.Jump = "ghost"
	if CheckHost(unknown, nil, []string{"realhost"})["jump"] == "" {
		t.Fatal("jump to a non-existent host should error")
	}
	self := base
	self.Jump = "a"
	if CheckHost(self, nil, []string{"a"})["jump"] == "" {
		t.Fatal("self-jump should error")
	}
}

func TestCheckHost_SSHOptions(t *testing.T) {
	base := HostForm{Name: "a", Host: "h"}
	bad := base
	bad.SSHOptions = "ServerAliveInterval 15" // no '='
	if CheckHost(bad, nil, nil)["sshOptions"] == "" {
		t.Fatal("option line without '=' should error")
	}
	dashP := base
	dashP.SSHOptions = "Foo=-p2222"
	if CheckHost(dashP, nil, nil)["sshOptions"] == "" {
		t.Fatal("option containing -p should error")
	}
}
