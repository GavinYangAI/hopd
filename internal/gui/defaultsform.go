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

// Apply parses the form and writes it into cfg: restart bounds and
// defaults.ssh_options. It returns a friendly error on bad input and does not
// mutate cfg on failure (durations are parsed before anything is written).
func (f DefaultsForm) Apply(cfg *config.Config) error {
	min, err := time.ParseDuration(strings.TrimSpace(f.RestartMin))
	if err != nil {
		return fmt.Errorf("重连最短间隔不是有效时长（如 2s、500ms）：%v", err)
	}
	max, err := time.ParseDuration(strings.TrimSpace(f.RestartMax))
	if err != nil {
		return fmt.Errorf("重连最长间隔不是有效时长（如 60s、2m）：%v", err)
	}
	opts, err := parseOptionLines(f.SSHOptions)
	if err != nil {
		return err
	}
	cfg.Restart.Min = min
	cfg.Restart.Max = max
	cfg.SetDefaultOptions(opts)
	return nil
}

// CheckDefaults validates the defaults form field-by-field and returns per-field
// messages (keys: restartMin, restartMax, sshOptions). It is pure so the dialog
// can call it live on every keystroke. An absent key means that field is valid.
func CheckDefaults(f DefaultsForm) FieldErrors {
	errs := FieldErrors{}

	min, minErr := time.ParseDuration(strings.TrimSpace(f.RestartMin))
	switch {
	case strings.TrimSpace(f.RestartMin) == "":
		errs["restartMin"] = "填重连最短间隔（如 2s）"
	case minErr != nil:
		errs["restartMin"] = "不是有效时长（如 2s、500ms）"
	case min <= 0:
		errs["restartMin"] = "要大于 0"
	}

	max, maxErr := time.ParseDuration(strings.TrimSpace(f.RestartMax))
	switch {
	case strings.TrimSpace(f.RestartMax) == "":
		errs["restartMax"] = "填重连最长间隔（如 60s）"
	case maxErr != nil:
		errs["restartMax"] = "不是有效时长（如 60s、2m）"
	case minErr == nil && max < min:
		errs["restartMax"] = "最长间隔要 ≥ 最短间隔"
	}

	if _, err := parseOptionLines(f.SSHOptions); err != nil {
		errs["sshOptions"] = err.Error()
	}
	return errs
}
