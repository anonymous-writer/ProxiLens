# Beta 10.10 build correction

Ubuntu/Debian `libpam0g-dev` may not provide `pam.pc`. The preflight now checks `/usr/include/security/pam_appl.h`, and the PAM Makefile links directly with `-lpam`.
