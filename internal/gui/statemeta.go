package gui

import (
	"strings"

	"github.com/GavinYangAI/hopd/internal/ipc"
)

// Tone is the visual colour family a state belongs to. It is UI-agnostic; the
// guiapp layer maps each tone to concrete theme colours.
type Tone string

const (
	ToneUp   Tone = "up"   // connected / healthy (green)
	ToneWarn Tone = "warn" // starting / retrying / needs-auth (amber)
	ToneErr  Tone = "err"  // unrecoverable error (red)
	ToneDown Tone = "down" // stopped / idle (grey)
)

// StateMeta is the human-facing description of a tunnel STATE.
type StateMeta struct {
	Label string // short Chinese label, e.g. "已连通"
	Blurb string // one-line explanation
	Tone  Tone
	Busy  bool // true while the state is transient (spinner-worthy)
}

// stateTable maps daemon STATE strings to their presentation.
var stateTable = map[string]StateMeta{
	"UP":         {Label: "已连通", Blurb: "转发就绪", Tone: ToneUp},
	"STARTING":   {Label: "连接中", Blurb: "正在建立隧道", Tone: ToneWarn, Busy: true},
	"RETRYING":   {Label: "重连中", Blurb: "断开后退避重连", Tone: ToneWarn, Busy: true},
	"NEEDS_AUTH": {Label: "待认证", Blurb: "需要你完成一次认证", Tone: ToneWarn},
	"ERROR":      {Label: "出错", Blurb: "不可恢复的错误", Tone: ToneErr},
	"DOWN":       {Label: "已停止", Blurb: "未启动", Tone: ToneDown},
}

// StateInfo returns the presentation metadata for a state, defaulting to DOWN
// for unknown values.
func StateInfo(state string) StateMeta {
	if m, ok := stateTable[state]; ok {
		return m
	}
	return stateTable["DOWN"]
}

// Summarize renders the one-line health summary shown in the window title and
// the tray dropdown, e.g. "2 已连通 · 1 重连中 · 1 出错".
func Summarize(snap []ipc.TunnelStatus) string {
	var up, busy, err, down int
	for _, t := range snap {
		switch StateInfo(t.State).Tone {
		case ToneUp:
			up++
		case ToneWarn:
			busy++
		case ToneErr:
			err++
		default:
			down++
		}
	}
	var parts []string
	if up > 0 {
		parts = append(parts, plural(up, "已连通"))
	}
	if busy > 0 {
		parts = append(parts, plural(busy, "连接中"))
	}
	if err > 0 {
		parts = append(parts, plural(err, "出错"))
	}
	if down > 0 {
		parts = append(parts, plural(down, "已停止"))
	}
	if len(parts) == 0 {
		return "暂无隧道"
	}
	return strings.Join(parts, " · ")
}

func plural(n int, label string) string {
	return itoa(n) + " " + label
}

// itoa is a tiny strconv.Itoa to keep this file import-light.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// OverallPhrase is the short status phrase shown beside the tray pill.
func OverallPhrase(o Overall) string {
	switch o {
	case OverallAllUp:
		return "全部正常"
	case OverallProblem:
		return "有隧道出错"
	case OverallBusy:
		return "部分连接中"
	case OverallDisconnected:
		return "未连接"
	default:
		return "运行中"
	}
}

// OverallTone maps the aggregate health to a tone for colouring the tray pill.
func OverallTone(o Overall) Tone {
	switch o {
	case OverallAllUp:
		return ToneUp
	case OverallProblem:
		return ToneErr
	case OverallBusy:
		return ToneWarn
	default:
		return ToneDown
	}
}
