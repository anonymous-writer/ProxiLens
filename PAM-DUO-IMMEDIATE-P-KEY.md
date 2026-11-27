# PAM: immediate Duo with P-key fallback

- `sudo` with a controlling terminal starts Duo immediately.
- Press `P` (without Enter) to stop waiting for Duo and continue with Ubuntu `common-auth`.
- The PAM module restores terminal settings before returning.
- GUI/non-TTY PAM clients retain a PAM-conversation password choice because they do not expose a controlling terminal for raw key capture.
- Debian packaging rebuilds the shared module from source via `make -C pam clean all`.

This change was checked with GCC `-Wall -Wextra -Werror -fsyntax-only` against
minimal PAM API declarations. It was not linked against the real libpam headers
or tested in a live sudo/Polkit session. Validate in a disposable VM first.
