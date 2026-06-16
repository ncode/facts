# Security Policy

## Supported Versions

Security fixes are provided for the latest tagged release and `main`.

| Version | Supported |
| ------- | --------- |
| `main` | Yes |
| `v0.0.2` | Yes |
| `v0.0.1` and older `v0` releases | No |

## Reporting a Vulnerability

Report vulnerabilities privately through GitHub private vulnerability
reporting for `ncode/facts`. Do not open a public issue for suspected
security problems.

Include enough detail to reproduce the issue:

- affected version or commit
- platform and architecture
- command, config, external fact, or API call involved
- expected impact
- proof of concept, if available

We aim to acknowledge reports within 7 days, provide status updates at least
every 30 days, and coordinate public disclosure after a fix is available.
If a report is accepted, we will coordinate the fix and disclosure in the
private report. If a report is declined, we will explain why.

## Scope

Security reports should focus on vulnerabilities in Facts itself, including
the `facts` CLI, the Go library, release artifacts, and handling of
operator-supplied inputs such as config files, environment facts, external
fact files, and external fact executables.

General host misconfiguration, unsupported platforms, and vulnerabilities in
third-party systems discovered by running Facts are outside this policy.
