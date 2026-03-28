# Security Policy

## Reporting Vulnerabilities

If you discover a security vulnerability in goro-pg, please report it responsibly:

**GitHub Security Advisory** (preferred): [Create a private security advisory](https://github.com/peter-trerotola/goro-pg/security/advisories/new)

**Do not open a public issue for security vulnerabilities.**

Please include:
- Description of the vulnerability
- Steps to reproduce
- Which defense tier(s) are bypassed (see below)
- Impact assessment

## Defense Model

This server enforces read-only access through four tiers:

| Tier | Mechanism | What it catches |
|------|-----------|-----------------|
| 1 | AST parser (`pg_query_go`) | Non-SELECT statements, SELECT INTO, FOR UPDATE, mutating CTEs |
| 2 | Connection-level `default_transaction_read_only=on` | Anything that slips past AST validation |
| 3 | Transaction-level `BEGIN READ ONLY` | Defense-in-depth for Tier 2 |
| 4 | PostgreSQL user grants (SELECT only) | Last line of defense at the database level |

## Adversarial Tests

The file `internal/guard/adversarial_test.go` contains ~200 test cases that attempt to bypass the read-only guard. If you find a bypass that is caught by Tiers 2-4 but not Tier 1, consider contributing it as a test case. See [Contributing Adversarial Tests](README.md#contributing-adversarial-tests) in the README.

## Supported Versions

Security fixes are applied to the latest release only. We recommend always running the latest version.
