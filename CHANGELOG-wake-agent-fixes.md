# Wake-Agent / Duo robustness changes

Changes in this patch:

- Wake-Agent logs now use the system's local timezone instead of forcing UTC.
- The initial logind session lookup gets a short 3-second startup grace period. A session that becomes available during that period is logged normally without an unnecessary startup warning. Later errors are still reported.
- Duo `auth_status` polling retries temporary network/transport errors with bounded backoff (1s, 2s, then 3s intervals) while the existing Push/context deadline remains in force.
- Non-network Duo API errors, malformed responses, explicit `deny`, and unexpected result values are not retried or accepted. Only `allow` completes authentication.
- Debian desktop entry sets `StartupNotify=false` to preserve the confirmed GNOME startup fix.

The same-session and locked-session checks before unlock are unchanged.
