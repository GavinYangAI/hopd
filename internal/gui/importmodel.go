package gui

import (
	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/sshconf"
)

// BuildHostsFromImport maps the selected ssh_config aliases into config.Host
// entries. Selection is by name: only ImportedHosts whose Name is in
// selectedNames are mapped; names in selectedNames with no matching ImportedHost
// are ignored. A host's ProxyJump becomes Host.Jump only when the referenced
// alias is itself selected; otherwise the jump is dropped (the dialog warns
// about dropped jumps separately). The function is pure over its inputs — it
// does not touch any config and does not check collisions with existing hosts.
func BuildHostsFromImport(imported []sshconf.ImportedHost, selectedNames []string) (map[string]config.Host, error) {
	selected := make(map[string]bool, len(selectedNames))
	for _, n := range selectedNames {
		selected[n] = true
	}

	out := map[string]config.Host{}
	for _, ih := range imported {
		if !selected[ih.Name] {
			continue
		}
		port := ih.Port
		if port == 0 {
			port = 22
		}
		h := config.Host{
			Host: ih.HostName,
			Port: port,
			User: ih.User,
			Key:  ih.IdentityFile,
		}
		// Keep the jump only if the target alias is also being imported.
		if ih.ProxyJump != "" && selected[ih.ProxyJump] {
			h.Jump = ih.ProxyJump
		}
		out[ih.Name] = h
	}
	return out, nil
}

// DroppedJumps reports, for each selected ImportedHost that had a ProxyJump,
// the jump target that will be dropped because it is not itself selected. The
// returned map is alias -> dropped jump target. Useful for a wizard warning.
func DroppedJumps(imported []sshconf.ImportedHost, selectedNames []string) map[string]string {
	selected := make(map[string]bool, len(selectedNames))
	for _, n := range selectedNames {
		selected[n] = true
	}
	dropped := map[string]string{}
	for _, ih := range imported {
		if !selected[ih.Name] || ih.ProxyJump == "" {
			continue
		}
		if !selected[ih.ProxyJump] {
			dropped[ih.Name] = ih.ProxyJump
		}
	}
	return dropped
}

// ExistingImportNames reports which imported aliases already exist as hosts in
// cfg, so the wizard can pre-mark/skip duplicates.
func ExistingImportNames(cfg *config.Config, imported []sshconf.ImportedHost) map[string]bool {
	dup := map[string]bool{}
	for _, ih := range imported {
		if _, ok := cfg.Host(ih.Name); ok {
			dup[ih.Name] = true
		}
	}
	return dup
}
