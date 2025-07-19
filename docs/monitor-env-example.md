# Monitor Environment Variable Configuration

This document shows how to use environment variables for sensitive credentials in the monitor configuration.

## Configuration File

Create a config file with environment variable placeholders:

```toml
# monitor-config.toml
[imap]
server = "mail.example.com"
port = 993
username = "${IMAP_USERNAME}"
password = "${IMAP_PASSWORD}"
use-ssl = true
retry-interval-seconds = 30

check-interval-seconds = 60

[[monitor]]
mailbox = "INBOX"
[[monitor.triggers]]
regex-pattern = "urgent.*alert"
command = "notify-send 'Email Alert'"

[[monitor.triggers]]
regex-pattern = "CRITICAL.*ERROR"
command = "echo 'Critical alert received'"
```

## Setting Environment Variables

Set your credentials as environment variables:

```bash
export IMAP_USERNAME="your-username@example.com"
export IMAP_PASSWORD="your-secure-password"
```

## Git Management

1. **Version control the config file** (it contains no secrets)
2. **Add credentials to `.env` or similar** (gitignored)
3. **Document required environment variables** in your README

## Supported Formats

The config loader supports both formats:
- `${VAR_NAME}` - recommended for clarity
- `$VAR_NAME` - also supported

## Behavior

- **Set variables**: Expanded to their values
- **Unset variables**: Preserved as `${VAR_NAME}` (won't cause errors)
- **Empty variables**: Expanded to empty strings

This allows you to safely version control configuration files while keeping credentials separate and secure.