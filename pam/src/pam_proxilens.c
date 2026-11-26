#define _GNU_SOURCE
#include <security/pam_modules.h>
#include <security/pam_appl.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/time.h>
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
    if (rc != PAM_SUCCESS || !resp) return PAM_CONV_ERR;
    *reply = resp[0].resp;
    free(resp);
    return PAM_SUCCESS;
}

PAM_EXTERN int pam_sm_authenticate(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    (void)flags; (void)argc; (void)argv;
    const char *user = NULL;
    if (pam_get_user(pamh, &user, NULL) != PAM_SUCCESS || !user || !*user || strlen(user) >= MAX_USER)
        return PAM_AUTH_ERR;

    char *choice = NULL;
    int rc = converse(pamh, PAM_PROMPT_ECHO_ON,
        "ProxiLens: [D]uo Push (Standard) oder [P] Ubuntu-Passwort (manuell waehlen)? ", &choice);
    if (rc != PAM_SUCCESS || !choice) return PAM_AUTH_ERR;
    int manual = (choice[0] == 'p' || choice[0] == 'P') && choice[1] == '\0';
    int duo = (choice[0] == '\0' || choice[0] == 'd' || choice[0] == 'D') && choice[1] == '\0';
    memset(choice, 0, strlen(choice)); free(choice);
    if (manual) return PAM_IGNORE; /* PAM stack must route IGNORE to common-auth. */
    if (!duo) return PAM_AUTH_ERR;

    char note[] = "ProxiLens: Duo Push wird angefordert. Bitte am Smartphone bestaetigen.";
    char *unused = NULL;
    (void)converse(pamh, PAM_TEXT_INFO, note, &unused);

    int fd = socket(AF_UNIX, SOCK_STREAM | SOCK_CLOEXEC, 0);
    if (fd < 0) return PAM_AUTH_ERR;
    struct timeval tv = {.tv_sec = 100, .tv_usec = 0};
    (void)setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
    (void)setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, &tv, sizeof(tv));
    struct sockaddr_un addr; memset(&addr, 0, sizeof(addr));
    addr.sun_family = AF_UNIX; strncpy(addr.sun_path, SOCKET_PATH, sizeof(addr.sun_path)-1);
    if (connect(fd, (struct sockaddr *)&addr, sizeof(addr)) != 0) { close(fd); return PAM_AUTH_ERR; }
    char req[MAX_USER + 2]; int n = snprintf(req, sizeof(req), "%s\n", user);
    if (n <= 1 || write(fd, req, (size_t)n) != n) { close(fd); return PAM_AUTH_ERR; }
    char answer[16] = {0}; ssize_t got = read(fd, answer, sizeof(answer)-1); close(fd);
    if (got > 0 && strcmp(answer, "ALLOW\n") == 0) return PAM_SUCCESS;
    return PAM_AUTH_ERR;
}

PAM_EXTERN int pam_sm_setcred(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    (void)pamh; (void)flags; (void)argc; (void)argv; return PAM_SUCCESS;
}
