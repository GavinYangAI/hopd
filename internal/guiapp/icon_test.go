package guiapp

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"

	"github.com/GavinYangAI/hopd/internal/gui"
)

func TestOverallColor(t *testing.T) {
	cases := map[gui.Overall]color.RGBA{
		gui.OverallAllUp:        {0x35, 0xd0, 0x6a, 0xff},
		gui.OverallProblem:      {0xff, 0x5e, 0x51, 0xff},
		gui.OverallBusy:         {0xff, 0xa7, 0x24, 0xff},
		gui.OverallNeutral:      {0x86, 0x8d, 0x9c, 0xff},
		gui.OverallDisconnected: {0x86, 0x8d, 0x9c, 0xff},
	}
	for o, want := range cases {
		if got := overallColor(o); got != want {
			t.Fatalf("overallColor(%v) = %v, want %v", o, got, want)
		}
	}
}

func TestIconFor_DecodesAsPNG(t *testing.T) {
	res := iconFor(gui.OverallAllUp)
	if res.Name() == "" {
		t.Fatal("resource should have a name")
	}
	img, err := png.Decode(bytes.NewReader(res.Content()))
	if err != nil {
		t.Fatalf("icon content is not valid PNG: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != iconSize || b.Dy() != iconSize {
		t.Fatalf("icon size = %dx%d, want %dx%d", b.Dx(), b.Dy(), iconSize, iconSize)
	}
}
