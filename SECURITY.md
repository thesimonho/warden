# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Warden, please report it responsibly. **Do not open a public issue.**

Use GitHub's [private vulnerability reporting](https://github.com/thesimonho/warden/security/advisories/new) to submit a report. This creates a private advisory visible only to you and the maintainers, where we can discuss the issue and collaborate on a fix before public disclosure.

## Scope

The following are in scope:

- Container escape or privilege escalation
- Network isolation bypass (restricted/air-gapped modes)
- Host filesystem access beyond configured bind mounts
- Authentication or authorization flaws in the API
- Cross-project data leakage
- Secrets exposure in logs, audit exports, or API responses

The following are out of scope:

- Vulnerabilities in Claude Code itself (report to [Anthropic](https://www.anthropic.com/responsible-disclosure))
- Vulnerabilities in Docker or Podman (report to the respective projects)
- Denial of service against the local Warden process (it runs on localhost by default)

## Supported Versions

Security fixes are applied to the latest release only. There is no LTS or backporting policy at this time.
