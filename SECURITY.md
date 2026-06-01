# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue for security problems.

Report privately via GitHub's **"Report a vulnerability"** button under the
repository's **Security** tab (Security Advisories). Include:

- what the issue is and the impact,
- steps to reproduce (or a proof of concept),
- the affected version / commit.

You'll get an acknowledgement, and a fix or mitigation will be coordinated
before any public disclosure.

## Scope notes

hopd wraps the system `ssh` binary and talks to a local daemon over a Unix
domain socket in the user's runtime directory. It does not store credentials:
it relies on your existing `~/.ssh/config`, ssh-agent, `known_hosts`, and any
2FA your hosts require. Reports about credential handling, the IPC socket's
permissions, or `ssh` argument construction are especially welcome.
