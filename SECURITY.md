# Security Policy

## Supported versions

Only the latest tagged release on `master` receives security updates.

| Version          | Supported |
|------------------|-----------|
| Latest `master`  | Yes       |
| Older tags       | No        |

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Report privately via GitHub's **Security Advisories** page:

`https://github.com/TyphoonMC/Crafty/security/advisories/new`

Or email the maintainers at the address listed in [CODEOWNERS](.github/CODEOWNERS).

Include:

- A clear description of the vulnerability
- Steps to reproduce (PoC preferred)
- Affected commit / version
- Your assessment of impact

We aim to:

- Acknowledge the report within **72 hours**
- Provide an initial assessment within **7 days**
- Publish a fix and advisory within **30 days** for confirmed high/critical issues

## Scope

In scope:

- The `cmd/crafty` binary
- Code under `internal/`
- Build, release and CI pipelines

Out of scope:

- Third-party dependencies (report upstream; we track via `govulncheck` and Dependabot)
- Denial of service requiring attacker-controlled local input to the game window
- Social engineering, physical attacks

## Disclosure

We follow coordinated disclosure. Credit is given to reporters unless they request otherwise.
