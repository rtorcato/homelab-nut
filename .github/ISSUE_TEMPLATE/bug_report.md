---
name: Bug report
about: Something isn't working as documented
title: ''
labels: bug
assignees: ''
---

## What happened

<!-- A clear description of what went wrong. -->

## What you expected

<!-- What should have happened instead. -->

## Steps to reproduce

1.
2.
3.

## Environment

- **UPS model:** <!-- e.g. APC Back-UPS ES 650G1, CyberPower CP1500PFCRM2U -->
- **NUT version:** <!-- output of `upsc -V` or `upsd -V` -->
- **OS / distro:** <!-- e.g. Debian 12, Ubuntu 24.04, Raspberry Pi OS -->
- **Arch:** <!-- output of `uname -m` -->
- **Script(s) involved:** <!-- e.g. setup-server.sh, ups-service.sh -->
- **Docker / bare-metal:** <!-- which setup path you used -->

## Logs

<details>
<summary>Relevant log output</summary>

```
<!-- Paste output from:
  journalctl -u nut-server -n 100
  journalctl -u ups-battery-shutdown -n 100
  /var/log/ups-battery-shutdown.log
-->
```

</details>

## Anything else?

<!-- Screenshots, config snippets (with secrets redacted), related issues, etc. -->
