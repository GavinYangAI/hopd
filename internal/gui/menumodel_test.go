package gui

import (
	"testing"

	"github.com/GavinYangAI/hopd/internal/ipc"
)

func TestBuildMenuModel_Disconnected(t *testing.T) {
	m := BuildMenuModel(nil, false)
	if m.Connected {
		t.Fatal("should be disconnected")
	}
	if m.Summary != "daemon 未运行" {
		t.Fatalf("summary = %q", m.Summary)
	}
	if len(m.Groups) != 0 {
		t.Fatalf("disconnected model should have no groups")
	}
}

func TestBuildMenuModel_GroupsAndLabels(t *testing.T) {
	snap := []ipc.TunnelStatus{
		{Name: "prod-db", Group: "prod", State: "UP"},
		{Name: "prod-redis", Group: "prod", State: "DOWN"},
		{Name: "stg-web", Group: "staging", State: "NEEDS_AUTH"},
		{Name: "stg-api", Group: "staging", State: "ERROR"},
	}
	m := BuildMenuModel(snap, true)
	if !m.Connected {
		t.Fatal("should be connected")
	}
	if m.Summary != "1 已连通 · 1 连接中 · 1 出错 · 1 已停止" {
		t.Fatalf("summary = %q", m.Summary)
	}
	if len(m.Groups) != 2 || m.Groups[0].Name != "prod" || m.Groups[1].Name != "staging" {
		t.Fatalf("groups = %+v (want prod, staging sorted)", m.Groups)
	}
	db := m.Groups[0].Items[0]
	if db.Name != "prod-db" || !db.Checked || db.Label != "prod-db · 已连通" {
		t.Fatalf("prod-db item = %+v", db)
	}
	redis := m.Groups[0].Items[1]
	if redis.Checked {
		t.Fatalf("DOWN tunnel should be unchecked: %+v", redis)
	}
	web := m.Groups[1].Items[0]
	if web.Label != "⚠ stg-web · 待认证" || !web.Checked {
		t.Fatalf("stg-web item = %+v", web)
	}
	api := m.Groups[1].Items[1]
	if api.Label != "✗ stg-api · 出错" {
		t.Fatalf("stg-api label = %q", api.Label)
	}
}
