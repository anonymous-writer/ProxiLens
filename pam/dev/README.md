# ProxiLens PAM input behavior

For `sudo` with a controlling terminal, the PAM module starts the policy/Duo
transaction immediately. It temporarily switches the controlling terminal to
non-canonical, no-echo input and watches for `P` without requiring Enter.
Pressing `P` returns `PAM_IGNORE`, allowing the following Ubuntu `common-auth`
stack to perform the normal password check. The original terminal settings are
restored on every exit path.

For GUI/non-TTY conversations (including Polkit), the module also starts Duo
immediately, then offers an explicit graphical fallback prompt. Entering `P`
and submitting the prompt returns `PAM_IGNORE` so the following Ubuntu
`common-auth` stack asks for the system password. Submitting an empty response
continues waiting for Duo. A GUI PAM conversation does not expose raw keyboard
key events to a PAM module, so this fallback necessarily uses the graphical
prompt's normal submit action rather than an Enter-free single-key shortcut.

The module must be rebuilt from `pam/src/pam_proxilens.c` by the Debian build
(`debian/rules` runs `make -C pam clean all`). Do not package or install a stale
`pam/pam_proxilens.so` from a previous build. Test in a disposable VM before
enabling PAM integrations on a production host.
