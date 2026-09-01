.PHONY: build gui gui-package test

# Version stamped into binaries: the exact tag on a release commit, else
# "<tag>-<n>-g<sha>[-dirty]", else the short sha. `hopd version` prints this.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/GavinYangAI/hopd/internal/version.Version=$(VERSION)
BUNDLE_VERSION ?= 0.1.0
BUNDLE_BUILD ?= 1

build:
	go build -ldflags "$(LDFLAGS)" -o hopd ./cmd/hopd

gui:
	go build -ldflags "$(LDFLAGS)" -o hopd-gui ./cmd/hopd-gui

# Package the menu-bar app as hopd-gui.app (requires `go install fyne.io/fyne/v2/cmd/fyne@v2.7.4`).
# Builds the bundle with the repo-root Icon.png, then sets LSUIElement so the
# app lives only in the menu bar (no Dock icon).
gui-package:
	fyne package --target darwin --src ./cmd/hopd-gui \
		--name hopd-gui --id com.gavinyangai.hopd.gui --icon "$(CURDIR)/Icon.png" \
		--appVersion "$(BUNDLE_VERSION)" --appBuild "$(BUNDLE_BUILD)" --release
	/usr/libexec/PlistBuddy -c "Add :LSUIElement bool true" "hopd-gui.app/Contents/Info.plist" \
		|| /usr/libexec/PlistBuddy -c "Set :LSUIElement true" "hopd-gui.app/Contents/Info.plist"
	@echo "Built hopd-gui.app (menu-bar only)."

test:
	go test ./...
