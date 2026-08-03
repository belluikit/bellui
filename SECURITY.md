# Security Policy

## Supported Versions

Minor releases will receive security updates and fixes until the next minor or major release.

| Version | Supported          |
| ------- | ------------------ |
| 2.6.x   | :white_check_mark: |
| < 2.6.0 | :x:                |

## Bundled Chromium (Linux webview)

On Linux the optional `webview` package renders with CEF and **the application
bundles the engine**, so its Chromium is only as current as the pin this
project ships — unlike every other platform, where the OS's own web engine is
updated by the OS. The bump cadence, owner, signal sources and the automated
staleness check are documented in
[docs/CEF-MAINTENANCE.md](docs/CEF-MAINTENANCE.md).

If you ship a Linux app embedding a web view, re-package against a current
bellui release when a Chromium security update lands: an app built months ago
carries the Chromium it was packaged with.

## Reporting a Vulnerability

Report security vulnerabilities using the [advisories](https://github.com/belluikit/bellui/security/advisories) page on GitHub.
The team of core developers will evaluate and address the issue as appropriate.
