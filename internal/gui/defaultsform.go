package gui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/GavinYangAI/hopd/internal/config"
)

// DefaultsForm is the editable, string-typed view of the global defaults block
// (defaults.restart + defaults.ssh_options) shown in the settings dialog.
type DefaultsForm struct {
	RestartMin string // duration string, e.g. "2s"
	RestartMax string // duration string, e.g. "1m0s"
	SSHOptions string // multiline key=value
}

// ToDefaultsForm reads a config's defaults into an editable form.
func ToDefaultsForm(cfg *config.Config) DefaultsForm {
	f := DefaultsForm{
		RestartMin: cfg.Restart.Min.String(),
		RestartMax: cfg.Restart.Max.String(),
	}
	opts := cfg.DefaultOptions()
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k + "=" + opts[k])
	}
	f.SSHOptions = b.String()
	return f
}

// parseOptionLines parses multiline "key=value" text into a map, rejecting the
// "-p" port option (which belongs on a host, not in global ssh_options) and
// lines missing an '='.
func parseOptionLines(text string) (map[string]string, error) {
	opts := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("无效的 ssh 选项 %q（要写成 key=value）", line)
		}
		key := strings.TrimSpace(k)
		if key == "-p" || strings.HasPrefix(key, "-p") {
			return nil, fmt.Errorf("不要在这里写 -p；端口请在主机里设置")
		}
		opts[key] = strings.TrimSpace(v)
	}
	return opts, nil
}

var _ = time.Second // keep time imported for Apply (Task 5)
