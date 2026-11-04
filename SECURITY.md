# Security Policy

## Supported versions

`homelab-nut` is pre-1.0. Only the latest release receives security fixes. Older releases are not patched; upgrade to the latest one.

## Reporting a vulnerability

Do **not** open a public GitHub issue for security vulnerabilities.

Instead, use [GitHub's private vulnerability reporting](https://github.com/rtorcato/homelab-nut/security/advisories/new) or email **rtorcato@me.com** with:

- A description of the vulnerability
- Steps to reproduce
- Potential impact

homelab-nut connects to your hosts over SSH and runs shell scripts on them, so reports about command injection, inventory or config parsing, SSH host-key handling, and the shutdown daemon are especially welcome.

You'll receive a response within 5 business days. Once the issue is confirmed and a fix is ready, a patch release will be published and the advisory will be made public.
