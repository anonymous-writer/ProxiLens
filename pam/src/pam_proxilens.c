#define _GNU_SOURCE
#include <security/pam_modules.h>
#include <security/pam_appl.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/time.h>
#include <sys/types.h>
#include <poll.h>
#include <termios.h>
#include <fcntl.h>
#include <unistd.h>
#include <errno.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>

#define SOCKET_PATH "/run/proxilens-policy/auth.sock"
#define MAX_USER 128

static int converse(pam_handle_t *pamh, int style, const char *msg, char **reply) {
    const struct pam_conv *conv = NULL;
    struct pam_message m, *mp = &m;
    struct pam_response *resp = NULL;
    int rc = pam_get_item(pamh, PAM_CONV, (const void **)&conv);
    if (rc != PAM_SUCCESS || !conv || !conv->conv) return PAM_CONV_ERR;
    m.msg_style = style; m.msg = msg;
    rc = conv->conv(1, (const struct pam_message **)&mp, &resp, conv->appdata_ptr);
    if (rc != PAM_SUCCESS || !resp) {
        if (resp) {
            if (resp[0].resp) { memset(resp[0].resp, 0, strlen(resp[0].resp)); free(resp[0].resp); }
            free(resp);
        }
        return PAM_CONV_ERR;
    }
    if (reply) {
        *reply = resp[0].resp;
        resp[0].resp = NULL;
    } else if (resp[0].resp) {
        memset(resp[0].resp, 0, strlen(resp[0].resp));
        free(resp[0].resp);
    }
    free(resp);
    return PAM_SUCCESS;
}

static int connect_policy(const char *service, const char *user) {
    int fd = socket(AF_UNIX, SOCK_STREAM | SOCK_CLOEXEC, 0);
    if (fd < 0) return -1;

    struct timeval tv = {.tv_sec = 100, .tv_usec = 0};
    (void)setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
    (void)setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, &tv, sizeof(tv));

    struct sockaddr_un addr;
    memset(&addr, 0, sizeof(addr));
    addr.sun_family = AF_UNIX;
    strncpy(addr.sun_path, SOCKET_PATH, sizeof(addr.sun_path) - 1);
    if (connect(fd, (struct sockaddr *)&addr, sizeof(addr)) != 0) {
        close(fd);
        return -1;
    }

    char req[MAX_USER + 16];
    int n = snprintf(req, sizeof(req), "%s\n%s\n", service, user);
    if (n <= 2 || (size_t)n >= sizeof(req)) {
        close(fd);
        return -1;
    }
    size_t sent = 0;
    while (sent < (size_t)n) {
        ssize_t w = write(fd, req + sent, (size_t)n - sent);
        if (w < 0 && errno == EINTR) continue;
        if (w <= 0) {
            memset(req, 0, sizeof(req));
            close(fd);
            return -1;
        }
        sent += (size_t)w;
    }
    memset(req, 0, sizeof(req));
    return fd;
}

/*
 * For sudo with a controlling terminal, start Duo first and watch for P
 * without requiring Enter. For GUI/non-TTY PAM conversations, start Duo
 * immediately and use the graphical PAM prompt for explicit password fallback.
 */
static int wait_for_duo_or_password(pam_handle_t *pamh, int fd, int ttyfd,
                                    const struct termios *saved, int raw_active) {
    struct pollfd pfds[2];
    nfds_t count = 1;
    pfds[0].fd = fd;
    pfds[0].events = POLLIN | POLLERR | POLLHUP;
    if (ttyfd >= 0 && raw_active) {
        pfds[1].fd = ttyfd;
        pfds[1].events = POLLIN | POLLERR | POLLHUP;
        count = 2;
        (void)converse(pamh, PAM_TEXT_INFO,
            "Duo Push gestartet. Mit P kannst du sofort zum Ubuntu-Passwort wechseln.", NULL);
    } else {
        (void)converse(pamh, PAM_TEXT_INFO,
            "Duo Push wird angefordert. Bitte am Smartphone bestaetigen.", NULL);
    }

    char answer[16] = {0};
    size_t received = 0;
    int result = PAM_AUTH_ERR;
    for (;;) {
        int pr = poll(pfds, count, 100000);
        if (pr < 0 && errno == EINTR) continue;
        if (pr <= 0) break;

        if (count == 2 && (pfds[1].revents & POLLIN)) {
            unsigned char key = 0;
            ssize_t nr = read(ttyfd, &key, 1);
            if (nr == 1 && (key == 'p' || key == 'P')) {
                (void)converse(pamh, PAM_TEXT_INFO,
                    "Wechsel zur Ubuntu-Passwortpruefung.", NULL);
                result = PAM_IGNORE;
                break;
            }
        }

        if (pfds[0].revents & (POLLIN | POLLERR | POLLHUP)) {
            ssize_t nr = read(fd, answer + received, sizeof(answer) - 1 - received);
            if (nr < 0 && errno == EINTR) continue;
            if (nr > 0) {
                received += (size_t)nr;
                if (memchr(answer, '\n', received) != NULL) {
                    if (received == 6 && memcmp(answer, "ALLOW\n", 6) == 0)
                        result = PAM_SUCCESS;
                    break;
                }
            } else {
                break;
            }
        }
    }
    if (raw_active && saved && ttyfd >= 0)
        (void)tcsetattr(ttyfd, TCSAFLUSH, saved);
    memset(answer, 0, sizeof(answer));
    return result;
}

PAM_EXTERN int pam_sm_authenticate(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    (void)flags; (void)argc; (void)argv;

    const char *service = NULL;
    const char *user = NULL;
    if (pam_get_item(pamh, PAM_SERVICE, (const void **)&service) != PAM_SUCCESS ||
        !service || (strcmp(service, "sudo") != 0 && strcmp(service, "polkit-1") != 0))
        return PAM_AUTH_ERR;
    if (pam_get_user(pamh, &user, NULL) != PAM_SUCCESS ||
        !user || !*user || strlen(user) >= MAX_USER)
        return PAM_AUTH_ERR;

    int ttyfd = -1;
    struct termios saved, raw;
    int raw_active = 0;
    if (strcmp(service, "sudo") == 0) {
        ttyfd = open("/dev/tty", O_RDWR | O_CLOEXEC | O_NOCTTY);
        if (ttyfd >= 0 && tcgetattr(ttyfd, &saved) == 0) {
            raw = saved;
            raw.c_lflag &= (tcflag_t)~(ICANON | ECHO);
            raw.c_cc[VMIN] = 1;
            raw.c_cc[VTIME] = 0;
            if (tcsetattr(ttyfd, TCSAFLUSH, &raw) == 0)
                raw_active = 1;
        }
        if (ttyfd >= 0 && !raw_active) {
            close(ttyfd);
            ttyfd = -1;
        }
    }

    /* A real TTY gets immediate Duo plus a single-key P escape hatch. */
    if (raw_active) {
        int fd = connect_policy(service, user);
        if (fd < 0) {
            (void)tcsetattr(ttyfd, TCSAFLUSH, &saved);
            close(ttyfd);
            return PAM_AUTH_ERR;
        }
        int result = wait_for_duo_or_password(pamh, fd, ttyfd, &saved, 1);
        close(fd);
        close(ttyfd);
        return result;
    }
    if (ttyfd >= 0) close(ttyfd);

    /* GUI/non-TTY (including Polkit): trigger Duo immediately.  The PAM
       conversation then provides the graphical, explicit password fallback.
       Unlike a TTY, a GUI agent cannot deliver a raw single key without its
       normal prompt/submit interaction. */
    int fd = connect_policy(service, user);
    if (fd < 0) return PAM_AUTH_ERR;

    char *choice = NULL;
    int rc = converse(pamh, PAM_PROMPT_ECHO_ON,
        "ProxiLens: Duo Push wurde gestartet. Ubuntu-Passwort statt Duo? P eingeben und bestaetigen; sonst Enter.",
        &choice);
    if (rc != PAM_SUCCESS || !choice) {
        close(fd);
        return PAM_AUTH_ERR;
    }
    int manual = (choice[0] == 'p' || choice[0] == 'P') && choice[1] == '\0';
    int duo = choice[0] == '\0';
    size_t choice_len = strlen(choice);
    memset(choice, 0, choice_len);
    free(choice);

    if (manual) {
        close(fd);
        return PAM_IGNORE; /* continue into the following Ubuntu common-auth */
    }
    if (!duo) {
        close(fd);
        return PAM_AUTH_ERR;
    }

    int result = wait_for_duo_or_password(pamh, fd, -1, NULL, 0);
    close(fd);
    return result;
}

PAM_EXTERN int pam_sm_setcred(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    (void)pamh; (void)flags; (void)argc; (void)argv; return PAM_SUCCESS;
}
