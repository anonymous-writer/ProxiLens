#define _GNU_SOURCE
#include <security/pam_appl.h>
#include <security/pam_modules.h>
#include <security/pam_ext.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <fcntl.h>
#include <unistd.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>

#define HELPER "/usr/lib/proxilens/proxilens-sudo-helper"

static int send_info(pam_handle_t *pamh, const char *text) {
    const struct pam_conv *conv = NULL;
    if (pam_get_item(pamh, PAM_CONV, (const void **)&conv) != PAM_SUCCESS || !conv || !conv->conv)
        return PAM_CONV_ERR;
    struct pam_message msg = { .msg_style = PAM_TEXT_INFO, .msg = text };
    const struct pam_message *msgs[1] = { &msg };
    struct pam_response *resp = NULL;
    int rc = conv->conv(1, msgs, &resp, conv->appdata_ptr);
    if (resp) { free(resp[0].resp); free(resp); }
    return rc == PAM_SUCCESS ? PAM_SUCCESS : PAM_CONV_ERR;
}

static int ask_password_fallback(pam_handle_t *pamh) {
    const struct pam_conv *conv = NULL;
    if (pam_get_item(pamh, PAM_CONV, (const void **)&conv) != PAM_SUCCESS || !conv || !conv->conv)
        return PAM_CONV_ERR;
    const char *prompt = "Duo Push war nicht erfolgreich oder hat nicht geantwortet. Ubuntu-Passwort verwenden? Eingabe P = Passwort, A = Abbrechen:";
    struct pam_message msg = { .msg_style = PAM_PROMPT_ECHO_ON, .msg = prompt };
    const struct pam_message *msgs[1] = { &msg };
    struct pam_response *resp = NULL;
    int rc = conv->conv(1, msgs, &resp, conv->appdata_ptr);
    if (rc != PAM_SUCCESS || !resp || !resp[0].resp) {
        if (resp) { free(resp[0].resp); free(resp); }
        return PAM_AUTH_ERR;
    }
    char choice = resp[0].resp[0];
    free(resp[0].resp);
    free(resp);
    if (choice == 'p' || choice == 'P') {
        (void)send_info(pamh, "ProxiLens: Ubuntu-Passwort als Backup ausgewählt.");
        return PAM_IGNORE; /* PAM stack continues with the standard common-auth password check. */
    }
    (void)send_info(pamh, "ProxiLens: Authentifizierung abgebrochen.");
    return PAM_AUTH_ERR;
}

static void set_pam_env(const char *key, const char *value) {
    if (value && *value) (void)setenv(key, value, 1);
}

PAM_EXTERN int pam_sm_authenticate(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    (void)flags; (void)argc; (void)argv;
    const char *user = NULL, *service = NULL, *tty = NULL, *ruser = NULL, *rhost = NULL;
    if (pam_get_user(pamh, &user, NULL) != PAM_SUCCESS || !user || !*user) return PAM_AUTH_ERR;
    (void)pam_get_item(pamh, PAM_SERVICE, (const void **)&service);
    (void)pam_get_item(pamh, PAM_TTY, (const void **)&tty);
    (void)pam_get_item(pamh, PAM_RUSER, (const void **)&ruser);
    (void)pam_get_item(pamh, PAM_RHOST, (const void **)&rhost);
    if (!service || strcmp(service, "sudo") != 0) return PAM_AUTH_ERR;

    if (send_info(pamh, "ProxiLens: Duo wird geprüft; falls erforderlich wird jetzt ein Push gestartet. Bitte auf dem Smartphone bestätigen.") != PAM_SUCCESS)
        return PAM_AUTH_ERR;

    pid_t pid = fork();
    if (pid < 0) return PAM_AUTH_ERR;
    if (pid == 0) {
        int nullfd = open("/dev/null", O_WRONLY);
        if (nullfd >= 0) { (void)dup2(nullfd, STDERR_FILENO); if (nullfd > STDERR_FILENO) close(nullfd); }
        set_pam_env("PAM_SERVICE", service);
        set_pam_env("PAM_TYPE", "auth");
        set_pam_env("PAM_USER", user);
        set_pam_env("PAM_TTY", tty);
        set_pam_env("PAM_RUSER", ruser);
        set_pam_env("PAM_RHOST", rhost);
        execl(HELPER, HELPER, "pam-auth", (char *)NULL);
        _exit(127);
    }
    int status = 0;
    while (waitpid(pid, &status, 0) < 0) { if (errno != EINTR) return PAM_AUTH_ERR; }
    if (WIFEXITED(status) && WEXITSTATUS(status) == 0) {
        (void)send_info(pamh, "ProxiLens: Duo-Freigabe erfolgreich.");
        return PAM_SUCCESS;
    }
    return ask_password_fallback(pamh);
}

PAM_EXTERN int pam_sm_setcred(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    (void)pamh; (void)flags; (void)argc; (void)argv;
    return PAM_SUCCESS;
}
