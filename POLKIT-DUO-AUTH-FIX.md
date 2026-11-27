# Polkit Duo authentication compatibility fix

This change targets the Polkit PAM conversation path only; the sudo raw-terminal path is left unchanged.

- Use `PAM_PROMPT_ECHO_OFF` for the Polkit fallback choice, matching the password-oriented Polkit authentication helper conversation.
- Do not emit a separate `PAM_TEXT_INFO` conversation from the Polkit wait path.
- Keep an empty response as Duo approval wait and `P` as `PAM_IGNORE`, allowing the following `common-auth` stack to perform Ubuntu password authentication.
- Log PAM-side outcome and broker-side peer/Duo/ALLOW outcome to the system journal (no Duo secrets are logged).

Build note: the PAM module must be rebuilt from `pam/src/pam_proxilens.c` by the Debian build (`make -C pam clean all`). The checked-in `.so` must not be copied as-is.

Validation limitation: this environment has no `libpam0g-dev` headers and cannot download the Go 1.27 toolchain, so a complete package build and live Polkit test were not possible here. Run the GitHub Actions DEB build and test on a non-critical account before replacing the current PAM module.
