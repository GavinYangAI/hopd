package guiapp

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/gui"
	"github.com/GavinYangAI/hopd/internal/ipc"
)

// svcByPort gives a friendly service name for well-known ports, shown as the
// target node's caption (e.g. 3389 → "RDP").
var svcByPort = map[string]string{
	"3389": "RDP", "5432": "PostgreSQL", "3306": "MySQL", "6379": "Redis",
	"8080": "Web 后台", "80": "HTTP", "443": "HTTPS", "5900": "VNC",
	"27017": "MongoDB", "22": "SSH", "6443": "K8s API", "9200": "Elasticsearch",
}

func svcFor(remote string) string {
	if i := strings.LastIndex(remote, ":"); i >= 0 {
		if name, ok := svcByPort[remote[i+1:]]; ok {
			return name
		}
	}
	return "目标"
}

// ---- small text helpers --------------------------------------------------

func text(s string, sz float32, c color.Color, style fyne.TextStyle) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = sz
	t.TextStyle = style
	return t
}

var (
	bold = fyne.TextStyle{Bold: true}
	mono = fyne.TextStyle{Monospace: true}
)

// roundRect builds a rounded, optionally stroked rectangle.
func roundRect(fill color.Color, radius, stroke float32, strokeColor color.Color) *canvas.Rectangle {
	r := canvas.NewRectangle(fill)
	r.CornerRadius = radius
	if stroke > 0 {
		r.StrokeColor = strokeColor
		r.StrokeWidth = stroke
	}
	return r
}

// ---- status badge --------------------------------------------------------

// statusBadge is a pill: a coloured dot + Chinese label on a soft tinted fill.
func statusBadge(state string) fyne.CanvasObject {
	meta := gui.StateInfo(state)
	tone := toneColor(meta.Tone)
	dot := canvas.NewCircle(tone)
	lbl := text(meta.Label, 12, tone, bold)
	row := container.New(layout12{gap: 6, padY: 3}, dot, lbl)
	bg := roundRect(toneSoft(meta.Tone), 999, 0, nil)
	return container.NewStack(bg, container.New(layoutPadXY{px: 9, py: 0}, row))
}

// layout12 lays a fixed dot then a baseline-aligned label horizontally.
type layout12 struct {
	gap  float32
	padY float32
}

func (l layout12) MinSize(objs []fyne.CanvasObject) fyne.Size {
	w, h := float32(0), float32(0)
	for i, o := range objs {
		m := o.MinSize()
		if i == 0 {
			m = fyne.NewSize(8, 8)
		}
		w += m.Width
		if i > 0 {
			w += l.gap
		}
		if m.Height > h {
			h = m.Height
		}
	}
	return fyne.NewSize(w, h+l.padY*2)
}
func (l layout12) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	x := float32(0)
	for i, o := range objs {
		m := o.MinSize()
		if i == 0 {
			m = fyne.NewSize(8, 8)
			o.Resize(m)
			o.Move(fyne.NewPos(x, (size.Height-m.Height)/2))
		} else {
			o.Resize(m)
			o.Move(fyne.NewPos(x, (size.Height-m.Height)/2))
		}
		x += m.Width + l.gap
	}
}

// layoutPadXY adds asymmetric padding around a single child.
type layoutPadXY struct{ px, py float32 }

func (l layoutPadXY) MinSize(objs []fyne.CanvasObject) fyne.Size {
	m := objs[0].MinSize()
	return fyne.NewSize(m.Width+l.px*2, m.Height+l.py*2)
}
func (l layoutPadXY) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	objs[0].Move(fyne.NewPos(l.px, l.py))
	objs[0].Resize(fyne.NewSize(size.Width-l.px*2, size.Height-l.py*2))
}

// ---- forward diagram -----------------------------------------------------

// diagNode is one box in the forward chain: a small grey caption over a mono value.
func diagNode(caption, value string, hop bool) fyne.CanvasObject {
	cap := text(caption, 10.5, pal.text3, fyne.TextStyle{})
	val := text(value, 12.5, pal.text1, mono)
	col := container.New(layoutStackV{gap: 2}, cap, val)
	fill := pal.surface2
	border := pal.border
	if hop {
		fill = pal.warnSoft
		border = pal.warnEdge
		cap.Color = pal.warn
	}
	bg := roundRect(fill, 8, 1, border)
	return container.NewStack(bg, container.New(layoutPadXY{px: 10, py: 6}, col))
}

// layoutStackV stacks children vertically, left-aligned, tight.
type layoutStackV struct{ gap float32 }

func (l layoutStackV) MinSize(objs []fyne.CanvasObject) fyne.Size {
	w, h := float32(0), float32(0)
	for i, o := range objs {
		m := o.MinSize()
		if m.Width > w {
			w = m.Width
		}
		h += m.Height
		if i > 0 {
			h += l.gap
		}
	}
	return fyne.NewSize(w, h)
}
func (l layoutStackV) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for _, o := range objs {
		m := o.MinSize()
		o.Resize(fyne.NewSize(size.Width, m.Height))
		o.Move(fyne.NewPos(0, y))
		y += m.Height + l.gap
	}
}

func arrow() fyne.CanvasObject {
	a := text("→", 14, pal.text3, fyne.TextStyle{})
	return container.New(layoutPadXY{px: 7, py: 0}, a)
}

// forwardDiagram renders 本机 :port → [中继 alias] → svc host:port for a status.
func forwardDiagram(t ipc.TunnelStatus) fyne.CanvasObject {
	nodes := []fyne.CanvasObject{
		diagNode("本机", ":"+localPart(t.Local), false),
	}
	if strings.TrimSpace(t.Via) != "" {
		nodes = append(nodes, arrow(), diagNode("中继", t.Via, true))
	}
	nodes = append(nodes, arrow(), diagNode(svcFor(t.Remote), valueOr(t.Remote, "—"), false))
	return container.NewHBox(nodes...)
}

func localPart(local string) string {
	if i := strings.LastIndex(local, ":"); i >= 0 {
		return local[i+1:]
	}
	if local == "" {
		return "—"
	}
	return local
}

func valueOr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// ---- tappable wrapper ----------------------------------------------------

type tappable struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
}

func newTappable(content fyne.CanvasObject, onTap func()) *tappable {
	t := &tappable{content: content, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappable) Tapped(*fyne.PointEvent)             { call(t.onTap) }
func (t *tappable) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.content) }

// ---- tunnel card ---------------------------------------------------------

// tunnelCard builds one selectable status card for a tunnel.
func tunnelCard(t ipc.TunnelStatus, selected bool, onSelect func(string)) fyne.CanvasObject {
	meta := gui.StateInfo(t.State)

	// top row: name + group chip ............... status badge
	name := text(t.Name, 15, pal.text1, bold)
	top := container.NewBorder(nil, nil, container.NewHBox(name, groupChip(t.Group)), statusBadge(t.State))

	// metrics row: 时长 / 重连 (+ optional CTA hint)
	metrics := container.NewHBox(
		metric("时长", uptimeText(t)),
		widget.NewLabel("  "),
		metric("重连", reconnText(t), t.Reconnects > 0),
	)
	var foot fyne.CanvasObject = metrics
	if hint := cardHint(t); hint != nil {
		foot = container.NewBorder(nil, nil, metrics, hint)
	}

	body := container.New(layoutStackV{gap: 11}, top, forwardDiagram(t), foot)

	// background + selection ring
	bgFill := pal.surface1
	strokeC := pal.border
	strokeW := float32(1)
	if selected {
		bgFill = pal.surface1h
		strokeC = pal.borderFocus
		strokeW = 2
	}
	bg := roundRect(bgFill, 12, strokeW, strokeC)

	// left accent edge in the state tone (Border stretches it to full height)
	edge := canvas.NewRectangle(toneColor(meta.Tone))
	edge.SetMinSize(fyne.NewSize(3, 0))

	inner := container.NewBorder(nil, nil, edge, nil,
		container.New(layoutPadXY{px: 14, py: 12}, body))
	card := container.NewStack(bg, inner)
	return newTappable(card, func() { onSelect(t.Name) })
}

func groupChip(name string) fyne.CanvasObject {
	if strings.TrimSpace(name) == "" {
		return container.NewWithoutLayout()
	}
	lbl := text(name, 11, pal.text2, fyne.TextStyle{})
	bg := roundRect(pal.surface2, 6, 1, pal.border)
	return container.NewStack(bg, container.New(layoutPadXY{px: 7, py: 2}, lbl))
}

func metric(key, value string, warnish ...bool) fyne.CanvasObject {
	k := text(key, 11, pal.text3, fyne.TextStyle{})
	vc := pal.text2
	if len(warnish) > 0 && warnish[0] {
		vc = pal.warn
	}
	v := text(value, 12.5, vc, mono)
	return container.NewHBox(k, v)
}

func uptimeText(t ipc.TunnelStatus) string {
	if t.UptimeSec <= 0 {
		return "—"
	}
	return fmtDuration(t.UptimeSec)
}

func reconnText(t ipc.TunnelStatus) string { return itoa(t.Reconnects) }

func fmtDuration(sec int64) string {
	h := sec / 3600
	m := (sec % 3600) / 60
	if h > 0 {
		return itoa64(h) + "h" + pad2(m) + "m"
	}
	s := sec % 60
	return itoa64(m) + "m" + pad2(s) + "s"
}

func pad2(n int64) string {
	if n < 10 {
		return "0" + itoa64(n)
	}
	return itoa64(n)
}

func itoa(n int) string { return itoa64(int64(n)) }
func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// cardHint returns the inline attention CTA for NEEDS_AUTH / ERROR cards.
func cardHint(t ipc.TunnelStatus) fyne.CanvasObject {
	switch t.State {
	case "NEEDS_AUTH":
		return text("待认证 · 需要完成一次认证", 12, pal.warn, bold)
	case "ERROR":
		msg := firstLine(t.LastError)
		if msg == "" {
			msg = "查看日志了解详情"
		}
		return text("⚠ "+truncate(msg, 30), 12, pal.err, fyne.TextStyle{})
	default:
		return nil
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
