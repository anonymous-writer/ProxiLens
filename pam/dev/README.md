# ProxiLens PAM input behavior

For `sudo` with a controlling terminal, the PAM module starts the policy/Duo
transaction immediately. It temporarily switches the controlling terminal to
non-canonical, no-echo input and watches for `P` while the policy socket is
pending. Pressing `P` returns `PAM_IGNORE`, allowing the following Ubuntu
`common-auth` stack to perform the normal password check. The original terminal
settings are restored on every exit path.

For GUI/non-TTY conversations (including Polkit), the module uses the trusted
PAM conversation to offer the password fallback; raw terminal key capture is
not available there.

The module must be rebuilt from `pam/src/pam_proxilens.c` by the Debian build
(`debian/rules` runs `make -C pam clean all`). Do not package or install a stale
`pam/pam_proxilens.so` from a previous build. Test in a disposable VM before
enabling PAM integrations on a production host.
