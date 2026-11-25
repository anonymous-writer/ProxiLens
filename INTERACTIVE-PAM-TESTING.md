# Interactive Duo-first PAM prototype

This source revision adds `pam/pam_proxilens.c` and switches the sudo and
Polkit authentication stacks to that module.

## Authentication flow

1. For the configured local user, PAM displays a status message and starts the
   existing root-owned ProxiLens Duo helper.
2. A successful Duo result authenticates and skips `common-auth`.
3. If Duo fails or times out, PAM asks for an explicit choice. Enter `P` to
   continue through Ubuntu's normal `common-auth` stack; any other response
   aborts authentication.
4. Other local users bypass the ProxiLens module and continue through Ubuntu's
   standard `common-auth` stack.

The choice is a PAM conversation prompt, not a custom GNOME button. The Polkit
agent on the target Ubuntu release must be tested to confirm how it renders
`PAM_TEXT_INFO` and `PAM_PROMPT_ECHO_ON` messages.

## Build dependency

The Debian build now requires `libpam0g-dev`. The module is installed to the
architecture-specific PAM security directory using `DEB_HOST_MULTIARCH`.

## Important validation status

The Go helper unit tests pass in the available environment when using its
locally installed Go toolchain with a temporary Go-version directive. The
original `go.mod` is restored unchanged. This environment does not contain the
PAM development headers, so `pam_proxilens.so` could not be compiled here; a
full Debian package build and a live sudo/Polkit conversation test have not
been completed. Treat this as a source/testing ZIP, not an install-ready or
validated binary package. Do not replace `/etc/pam.d/sudo` or
`/etc/pam.d/polkit-1` manually with this prototype.

Before deployment, build on Ubuntu with `libpam0g-dev` installed, then test
from a second administrator session with a recovery path available. Verify
both Duo success and explicit password fallback for `sudo` and Polkit, plus
cancel and timeout behavior. The existing Polkit systemd network drop-in and
shared Duo grant implementation remain included.
