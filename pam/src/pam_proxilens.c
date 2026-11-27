#define _GNU_SOURCE
#include <security/pam_modules.h>
#include <security/pam_appl.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/time.h>
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

/* sudo only: start Duo immediately and watch for P without Enter on a real TTY. */
static int sudo_immediate_tty(pam_handle_t *pamh, const char *user) {
    int ttyfd = open("/dev/tty", O_RDWR | O_CLOEXEC | O_NOCTTY);
    if (ttyfd < 0) return PAM_IGNORE; /* retain the established conversation fallback */

    struct termios saved, raw;
    if (tcgetattr(ttyfd, &saved) != 0) {
        close(ttyfd);
        return PAM_IGNORE;
    }
    raw = saved;
    raw.c_lflag &= (tcflag_t)~(ICANON | ECHO);
    raw.c_cc[VMIN] = 1;
    raw.c_cc[VTIME] = 0;
    if (tcsetattr(ttyfd, TCSAFLUSH, &raw) != 0) {
        close(ttyfd);
        return PAM_IGNORE;
    }

    int fd = connect_policy("sudo", user);
    if (fd < 0) {
        (void)tcsetattr(ttyfd, TCSAFLUSH, &saved);
        close(ttyfd);
        return PAM_AUTH_ERR;
    }

    char note[] = "Duo Push gestartet. Mit P kannst du sofort zum Ubuntu-Passwort wechseln.";
    (void)converse(pamh, PAM_TEXT_INFO, note, NULL);

    struct pollfd pfds[2] = {
        {.fd = fd, .events = POLLIN | POLLERR | POLLHUP},
        {.fd = ttyfd, .events = POLLIN | POLLERR | POLLHUP}
    };
    char answer[16] = {0};
    size_t received = 0;
    int result = PAM_AUTH_ERR;
    for (;;) {
        int pr = poll(pfds, 2, 100000);
        if (pr < 0 && errno == EINTR) continue;
        if (pr <= 0) break;
        if (pfds[1].revents & POLLIN) {
            unsigned char key = 0;
            if (read(ttyfd, &key, 1) == 1 && (key == 'p' || key == 'P')) {
                char msg[] = "Wechsel zur Ubuntu-Passwortpruefung.";
                (void)converse(pamh, PAM_TEXT_INFO, msg, NULL);
                result = PAM_IGNORE;
                break;
            }
        }
        if (pfds[0].revents & (POLLIN | POLLERR | POLLHUP)) {
            ssize_t nr = read(fd, answer + received, sizeof(answer) - 1 - received);
            if (nr < 0 && errno == EINTR) continue;
            if (nr <= 0) break;
            received += (size_t)nr;
            if (memchr(answer, '\n', received) != NULL) {
                if (received == 6 && memcmp(answer, "ALLOW\n", 6) == 0)
                    result = PAM_SUCCESS;
                break;
            }
        }
    }
    (void)tcsetattr(ttyfd, TCSAFLUSH, &saved);
    memset(answer, 0, sizeof(answer));
    close(fd);
    close(ttyfd);
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

    /* sudo: Duo starts immediately; P is read directly from the controlling TTY. */
    if (strcmp(service, "sudo") == 0) {
        int immediate = sudo_immediate_tty(pamh, user);
        if (immediate != PAM_IGNORE) return immediate;
    }

    /*
     * Polkit: start Duo Push BEFORE displaying the familiar D/P menu.
     * Keep the broker connection open while the agent displays the prompt.
     * D accepts the already-running Push; P closes the request and lets the
     * following common-auth stack perform the Ubuntu password check.
     * This preserves the known-working Polkit conversation and removes only
     * the requirement to press D before the Push starts.
     */
    if (strcmp(service, "polkit-1") == 0) {
        int fd = connect_policy("polkit-1", user);
        if (fd < 0) return PAM_IGNORE;

        char *choice = NULL;
        const char *prompt =
            "ProxiLens: Duo Push laeuft bereits. [D] bestaetigen oder [P] Ubuntu-Passwort: ";
        int rc = converse(pamh, PAM_PROMPT_ECHO_ON, prompt, &choice);
        if (rc != PAM_SUCCESS || !choice) {
            close(fd);
            return PAM_CONV_ERR;
        }

        int manual = (choice[0] == 'p' || choice[0] == 'P') && choice[1] == '\0';
        int duo = (choice[0] == '\0' || choice[0] == 'd' || choice[0] == 'D') &&
                  choice[1] == '\0';
        size_t choice_len = strlen(choice);
        memset(choice, 0, choice_len);
        free(choice);

        if (manual) {
            close(fd);
            return PAM_IGNORE;
        }
        if (!duo) {
            close(fd);
            return PAM_AUTH_ERR;
        }

        char answer[16] = {0};
        size_t received = 0;
        while (received < sizeof(answer) - 1) {
            ssize_t r = read(fd, answer + received, sizeof(answer) - 1 - received);
            if (r < 0 && errno == EINTR) continue;
            if (r <= 0) break;
            received += (size_t)r;
            if (memchr(answer, '\n', received) != NULL) break;
        }
        close(fd);
        if (received == 6 && memcmp(answer, "ALLOW\n", 6) == 0) {
            memset(answer, 0, sizeof(answer));
            return PAM_SUCCESS;
        }
        memset(answer, 0, sizeof(answer));
        return PAM_IGNORE;
    }

    /* Non-Polkit fallback path retains the established D/P conversation. */
    char *choice = NULL;
    const char *prompt = "ProxiLens: [D]uo Push (Standard) oder [P] Ubuntu-Passwort (manuell waehlen)? ";
    int rc = converse(pamh, PAM_PROMPT_ECHO_ON, prompt, &choice);
    if (rc != PAM_SUCCESS || !choice) return PAM_AUTH_ERR;
    int manual = (choice[0] == 'p' || choice[0] == 'P') && choice[1] == '\0';
    int duo = (choice[0] == '\0' || choice[0] == 'd' || choice[0] == 'D') &&
              choice[1] == '\0';
    size_t choice_len = strlen(choice);
    memset(choice, 0, choice_len);
    free(choice);
    if (manual) return PAM_IGNORE;
    if (!duo) return PAM_AUTH_ERR;

    char note[] = "ProxiLens: Duo Push wird angefordert. Bitte am Smartphone bestaetigen.";
    (void)converse(pamh, PAM_TEXT_INFO, note, NULL);
    int fd = connect_policy(service, user);
    if (fd < 0) return PAM_AUTH_ERR;
    char answer[16] = {0};
    size_t received = 0;
    while (received < sizeof(answer) - 1) {
        ssize_t r = read(fd, answer + received, sizeof(answer) - 1 - received);
        if (r < 0 && errno == EINTR) continue;
        if (r <= 0) break;
        received += (size_t)r;
        if (memchr(answer, '\n', received) != NULL) break;
    }
    close(fd);
    if (received == 6 && memcmp(answer, "ALLOW\n", 6) == 0)
        return PAM_SUCCESS;
    memset(answer, 0, sizeof(answer));
    return PAM_AUTH_ERR;
}

PAM_EXTERN int pam_sm_setcred(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    (void)pamh; (void)flags; (void)argc; (void)argv; return PAM_SUCCESS;
}
