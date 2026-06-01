package guiapp

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/GavinYangAI/hopd/internal/gui"
)

// palette is the "石板蓝灰 (slate)" colour set from the design prototype. The
// custom Fyne theme maps these onto Fyne's named colours, and the card / form /
// icon code reads the same struct directly for the bits Fyne can't express via
// named colours (status tones, accent edges, soft fills).
type palette struct {
	surface0, surface1, surface1h, surface2, surfacePop color.Color
	barTop, barBot, logBg                               color.Color
	text1, text2, text3                                 color.Color
	accent, accentH, accentSoft, accentBg               color.Color
	up, upSoft                                          color.Color
	warn, warnSoft, warnEdge                            color.Color
	err, errSoft                                        color.Color
	down, downSoft                                      color.Color
	hover, ghostHover                                   color.Color
	border, borderStrong, borderFocus                   color.Color
	overlay                                             color.Color
}

var transparent = color.NRGBA{}

func rgb(hex uint32) color.Color {
	return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 0xff}
}

func rgba(hex uint32, a uint8) color.Color {
	return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: a}
}

// pal is the single live palette. Default is the "雾灰浅色 (mist)" light theme,
// matching the prototype's default (index.html: "clean white / light").
var pal = mistPalette

// activeTheme is the name of the currently applied palette.
var activeTheme = "mist"

// themeOrder is the selectable themes, in menu order, with Chinese labels.
var themeOrder = []struct{ Name, Label string }{
	{"mist", "雾灰浅色"},
	{"slate", "石板蓝灰"},
	{"graphite", "暖炭灰"},
	{"indigo", "午夜靛蓝"},
}

var palettes = map[string]palette{
	"mist":     mistPalette,
	"slate":    slatePalette,
	"graphite": graphitePalette,
	"indigo":   indigoPalette,
}

// setPalette switches the live palette by name (falling back to mist). It only
// swaps colours; callers must rebuild themed content to see the change.
func setPalette(name string) {
	p, ok := palettes[name]
	if !ok {
		name, p = "mist", mistPalette
	}
	pal = p
	activeTheme = name
}

// slatePalette — "石板蓝灰", dark blue-grey.
var slatePalette = palette{
	surface0: rgb(0x272c39), surface1: rgb(0x2e3441), surface1h: rgb(0x363d4d),
	surface2: rgb(0x3a4151), surfacePop: rgb(0x30364a),
	barTop: rgb(0x2b303d), barBot: rgb(0x282d3a), logBg: rgb(0x1e2230),
	text1: rgb(0xf1f3f7), text2: rgb(0x9aa0ad), text3: rgb(0x6c7280),
	accent: rgb(0x0a84ff), accentH: rgb(0x3a9bff), accentSoft: rgba(0x0a84ff, 0x2b), accentBg: rgba(0x0a84ff, 0x14),
	up: rgb(0x35d06a), upSoft: rgba(0x35d06a, 0x26),
	warn: rgb(0xffa724), warnSoft: rgba(0xffa724, 0x29), warnEdge: rgba(0xffa724, 0x3a),
	err: rgb(0xff5e51), errSoft: rgba(0xff5e51, 0x29),
	down: rgb(0x868d9c), downSoft: rgba(0x868d9c, 0x29),
	hover: rgba(0xffffff, 0x12), ghostHover: rgb(0x454d60),
	border: rgba(0xffffff, 0x16), borderStrong: rgba(0xffffff, 0x24), borderFocus: rgba(0x0a84ff, 0x99),
	overlay: rgba(0x000000, 0x80),
}

// graphitePalette — "暖炭灰", warm dark.
var graphitePalette = palette{
	surface0: rgb(0x302e2a), surface1: rgb(0x38352f), surface1h: rgb(0x403c35),
	surface2: rgb(0x45413a), surfacePop: rgb(0x3a362f),
	barTop: rgb(0x34312b), barBot: rgb(0x312e28), logBg: rgb(0x221f1b),
	text1: rgb(0xf5f2ec), text2: rgb(0xa8a298), text3: rgb(0x79736a),
	accent: rgb(0x0a84ff), accentH: rgb(0x3a9bff), accentSoft: rgba(0x0a84ff, 0x29), accentBg: rgba(0x0a84ff, 0x14),
	up: rgb(0x3fcf6e), upSoft: rgba(0x3fcf6e, 0x26),
	warn: rgb(0xf5a623), warnSoft: rgba(0xf5a623, 0x29), warnEdge: rgba(0xf5a623, 0x3a),
	err: rgb(0xff6155), errSoft: rgba(0xff6155, 0x29),
	down: rgb(0x928c82), downSoft: rgba(0x928c82, 0x29),
	hover: rgba(0xffffff, 0x0f), ghostHover: rgb(0x514c44),
	border: rgba(0xffffff, 0x13), borderStrong: rgba(0xffffff, 0x21), borderFocus: rgba(0x0a84ff, 0x99),
	overlay: rgba(0x000000, 0x80),
}

// indigoPalette — "午夜靛蓝", deep indigo.
var indigoPalette = palette{
	surface0: rgb(0x1f2338), surface1: rgb(0x262b44), surface1h: rgb(0x2e3450),
	surface2: rgb(0x333a5a), surfacePop: rgb(0x282d48),
	barTop: rgb(0x242942), barBot: rgb(0x212546), logBg: rgb(0x15182b),
	text1: rgb(0xeef0fb), text2: rgb(0x9da3c0), text3: rgb(0x6a7099),
	accent: rgb(0x5b8cff), accentH: rgb(0x7da4ff), accentSoft: rgba(0x5b8cff, 0x2e), accentBg: rgba(0x5b8cff, 0x14),
	up: rgb(0x3ed873), upSoft: rgba(0x3ed873, 0x26),
	warn: rgb(0xffb02e), warnSoft: rgba(0xffb02e, 0x2b), warnEdge: rgba(0xffb02e, 0x3a),
	err: rgb(0xff6a5e), errSoft: rgba(0xff6a5e, 0x2b),
	down: rgb(0x7f86a6), downSoft: rgba(0x7f86a6, 0x2b),
	hover: rgba(0xffffff, 0x14), ghostHover: rgb(0x3d4570),
	border: rgba(0xffffff, 0x17), borderStrong: rgba(0xffffff, 0x26), borderFocus: rgba(0x5b8cff, 0x99),
	overlay: rgba(0x000000, 0x80),
}

// mistPalette is the light, white-dominant theme from app-theme.jsx.
var mistPalette = palette{
	surface0:     rgb(0xffffff),
	surface1:     rgb(0xf6f8fb),
	surface1h:    rgb(0xeef1f6),
	surface2:     rgb(0xeef1f6),
	surfacePop:   rgb(0xffffff),
	barTop:       rgb(0xf6f8fb),
	barBot:       rgb(0xf1f4f8),
	logBg:        rgb(0xf1f4f8),
	text1:        rgb(0x1b1e26),
	text2:        rgb(0x586070),
	text3:        rgb(0x97a0ad),
	accent:       rgb(0x0a72e6),
	accentH:      rgb(0x0a84ff),
	accentSoft:   rgba(0x0a84ff, 0x1f),
	accentBg:     rgba(0x0a72e6, 0x14),
	up:           rgb(0x1f9d54),
	upSoft:       rgba(0x1f9d54, 0x21),
	warn:         rgb(0xbb7900),
	warnSoft:     rgba(0xbb7900, 0x24),
	warnEdge:     rgba(0xbb7900, 0x40),
	err:          rgb(0xd83b2e),
	errSoft:      rgba(0xd83b2e, 0x1f),
	down:         rgb(0x717885),
	downSoft:     rgba(0x717885, 0x21),
	hover:        rgba(0x0f172a, 0x0d),
	ghostHover:   rgb(0xe4e8ef),
	border:       rgba(0x0f172a, 0x1a),
	borderStrong: rgba(0x0f172a, 0x2b),
	borderFocus:  rgba(0x0a72e6, 0x80),
	overlay:      rgba(0x000000, 0x80),
}

// toneColor maps a gui.Tone to its solid colour.
func toneColor(t gui.Tone) color.Color {
	switch t {
	case gui.ToneUp:
		return pal.up
	case gui.ToneWarn:
		return pal.warn
	case gui.ToneErr:
		return pal.err
	default:
		return pal.down
	}
}

// toneSoft maps a gui.Tone to its soft (translucent) fill.
func toneSoft(t gui.Tone) color.Color {
	switch t {
	case gui.ToneUp:
		return pal.upSoft
	case gui.ToneWarn:
		return pal.warnSoft
	case gui.ToneErr:
		return pal.errSoft
	default:
		return pal.downSoft
	}
}

// hopdTheme is a fixed dark "slate" theme. It ignores the system light/dark
// variant on purpose: hopd is a menu-bar utility with a single curated look.
type hopdTheme struct{}

func (hopdTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return pal.surface0
	case theme.ColorNameForeground:
		return pal.text1
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnError, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
		return rgb(0xffffff)
	case theme.ColorNameButton:
		return pal.surface2
	case theme.ColorNameDisabledButton:
		return pal.surface1
	case theme.ColorNameDisabled:
		return pal.text3
	case theme.ColorNamePlaceHolder:
		return pal.text3
	case theme.ColorNameInputBackground:
		return pal.surface2
	case theme.ColorNameInputBorder:
		return pal.border
	case theme.ColorNamePrimary:
		return pal.accent
	case theme.ColorNameHyperlink:
		return pal.accentH
	case theme.ColorNameSuccess:
		return pal.up
	case theme.ColorNameWarning:
		return pal.warn
	case theme.ColorNameError:
		return pal.err
	case theme.ColorNameHover:
		return pal.hover
	case theme.ColorNameSeparator:
		return pal.border
	case theme.ColorNameSelection:
		return pal.accentSoft
	case theme.ColorNameFocus:
		return pal.borderFocus
	case theme.ColorNamePressed:
		return pal.hover
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return pal.surfacePop
	case theme.ColorNameHeaderBackground:
		return pal.barTop
	case theme.ColorNameShadow:
		return rgba(0x000000, 0x66)
	case theme.ColorNameScrollBar:
		return rgba(0xffffff, 0x33)
	case theme.ColorNameScrollBarBackground:
		return rgba(0x000000, 0x00)
	default:
		return theme.DefaultTheme().Color(name, baseVariant())
	}
}

// baseVariant picks the Fyne base variant for unhandled colour names: light for
// the mist theme, dark for the rest.
func baseVariant() fyne.ThemeVariant {
	if activeTheme == "mist" {
		return theme.VariantLight
	}
	return theme.VariantDark
}

func (hopdTheme) Font(s fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(s) }
func (hopdTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (hopdTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 13.5
	case theme.SizeNamePadding:
		return 5
	case theme.SizeNameInnerPadding:
		return 9
	case theme.SizeNameInputRadius:
		return 9
	case theme.SizeNameSelectionRadius:
		return 7
	case theme.SizeNameScrollBar:
		return 10
	case theme.SizeNameScrollBarSmall:
		return 4
	default:
		return theme.DefaultTheme().Size(name)
	}
}
