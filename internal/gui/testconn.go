package gui

import (
	"bufio"
	"bytes"
	"strings"
)

// HostKey is one host-key fingerprint surfaced by a test connection, so the
// trust dialog can show what was added to ~/.ssh/known_hosts.
type HostKey struct {
	Host        string // hostname / IP as ssh names it
	Algo        string // ED25519 / RSA / ECDSA …
	Fingerprint string // SHA256:… ("" if ssh didn't print one)
}

// parseHostKeys extracts host-key info from ssh stderr produced under
// StrictHostKeyChecking=accept-new. It pairs each
//
//	Warning: Permanently added '<host>' (<ALGO>) to the list of known hosts.
//
// line with a following
//
//	<ALGO> key fingerprint is SHA256:…
//
// line when present.
func parseHostKeys(stderr []byte) []HostKey {
	var keys []HostKey
	fps := map[string]string{} // ALGO -> fingerprint
	sc := bufio.NewScanner(bytes.NewReader(stderr))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.Index(line, "key fingerprint is "); i >= 0 {
			algo := strings.ToUpper(strings.TrimSpace(line[:i]))
			fp := strings.TrimRight(strings.TrimSpace(line[i+len("key fingerprint is "):]), ".")
			fps[algo] = fp
		}
	}
	sc = bufio.NewScanner(bytes.NewReader(stderr))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "Warning: Permanently added") {
			continue
		}
		host := between(line, "'", "'")
		algo := strings.ToUpper(between(line, "(", ")"))
		keys = append(keys, HostKey{Host: host, Algo: algo, Fingerprint: fps[algo]})
	}
	return keys
}

// between returns the substring between the first open and the next close after
// it; "" if not found.
func between(s, open, close string) string {
	i := strings.Index(s, open)
	if i < 0 {
		return ""
	}
	rest := s[i+len(open):]
	j := strings.Index(rest, close)
	if j < 0 {
		return ""
	}
	return rest[:j]
}
