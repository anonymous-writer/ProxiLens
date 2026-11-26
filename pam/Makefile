CC ?= cc
CFLAGS ?= -O2 -fPIC -Wall -Wextra -Werror -D_FORTIFY_SOURCE=2
# Debian/Ubuntu libpam0g-dev may not provide pam.pc; the header is in the standard include path.
PAM_CFLAGS ?=
PAM_LIBS ?= -lpam
all: pam_proxilens.so
pam_proxilens.so: src/pam_proxilens.c
	$(CC) $(CFLAGS) $(PAM_CFLAGS) -shared -Wl,-z,relro,-z,now -o $@ $< $(PAM_LIBS)
clean:
	rm -f pam_proxilens.so
.PHONY: all clean
