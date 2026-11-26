#define _GNU_SOURCE
#include <security/pam_appl.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <termios.h>
#include <unistd.h>
#include <pwd.h>
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <limits.h>
#include <signal.h>

#define SERVICE_NAME "proxilens-test"
#define MAX_PASSWORD 4096

static volatile sig_atomic_t interrupted = 0;
static void note_signal(int signo) { (void)signo; interrupted = 1; }

static const char *pam_conf =
    "# ProxiLens isolated PAM diagnostic; generated in a private /run directory.\n"
    "auth    requisite pam_unix.so\n"
    "account requisite pam_unix.so\n";

static int write_all(int fd, const char *buf, size_t len) {
    size_t off = 0;
    while (off < len) {
        ssize_t n = write(fd, buf + off, len - off);
        if (n < 0) { if (errno == EINTR) continue; return -1; }
        if (n == 0) return -1;
        off += (size_t)n;
    }
    return 0;
}

static void wipe_free(char *p) {
    if (p) {
        explicit_bzero(p, strlen(p));
        free(p);
    }
}

static int read_secret(char **out) {
    int fd = open("/dev/tty", O_RDWR | O_CLOEXEC | O_NOCTTY);
    if (fd < 0) return -1;
    struct termios oldt, newt;
    if (tcgetattr(fd, &oldt) != 0) { close(fd); return -1; }
    struct sigaction sa, old_actions[5];
    const int signals[5] = { SIGINT, SIGTERM, SIGHUP, SIGQUIT, SIGTSTP };
    memset(&sa, 0, sizeof(sa));
    sa.sa_handler = note_signal;
    sigemptyset(&sa.sa_mask);
    interrupted = 0;
    for (size_t i = 0; i < 5; i++) {
        if (sigaction(signals[i], &sa, &old_actions[i]) != 0) {
            while (i > 0) { i--; (void)sigaction(signals[i], &old_actions[i], NULL); }
            close(fd);
            return -1;
        }
    }
    newt = oldt;
    newt.c_lflag &= (tcflag_t)~ECHO;
    if (tcsetattr(fd, TCSAFLUSH, &newt) != 0) {
        for (size_t i = 0; i < 5; i++) (void)sigaction(signals[i], &old_actions[i], NULL);
        close(fd);
        return -1;
    }

    const char prompt[] = "Ubuntu-Kontopasswort (Eingabe verborgen): ";
    if (write_all(fd, prompt, sizeof(prompt) - 1) != 0) {
        (void)tcsetattr(fd, TCSAFLUSH, &oldt);
        for (size_t i = 0; i < 5; i++) (void)sigaction(signals[i], &old_actions[i], NULL);
        close(fd);
        return -1;
    }
    char *buf = calloc(MAX_PASSWORD + 1, 1);
    if (!buf) {
        (void)tcsetattr(fd, TCSAFLUSH, &oldt);
        for (size_t i = 0; i < 5; i++) (void)sigaction(signals[i], &old_actions[i], NULL);
        close(fd);
        return -1;
    }
    size_t n = 0;
    int ok = 1;
    for (;;) {
        if (interrupted) { ok = 0; break; }
        unsigned char ch;
        ssize_t got = read(fd, &ch, 1);
        if (got != 1) { if (got < 0 && errno == EINTR && !interrupted) continue; ok = 0; break; }
        if (ch == '\n' || ch == '\r') break;
        if (ch == 0 || n >= MAX_PASSWORD) { ok = 0; break; }
        buf[n++] = (char)ch;
    }
    (void)tcsetattr(fd, TCSAFLUSH, &oldt);
    (void)write_all(fd, "\n", 1);
    for (size_t i = 0; i < 5; i++) (void)sigaction(signals[i], &old_actions[i], NULL);
    close(fd);
    if (!ok || n == 0) { wipe_free(buf); return -1; }
    buf[n] = '\0';
    *out = buf;
    return 0;
}

static int conversation(int count, const struct pam_message **messages,
                        struct pam_response **responses, void *data) {
    (void)data;
    if (count <= 0 || count > 8 || !messages || !responses) return PAM_CONV_ERR;
    struct pam_response *reply = calloc((size_t)count, sizeof(*reply));
    if (!reply) return PAM_BUF_ERR;
    for (int i = 0; i < count; i++) {
        if (!messages[i]) goto fail;
        switch (messages[i]->msg_style) {
        case PAM_PROMPT_ECHO_OFF:
            if (read_secret(&reply[i].resp) != 0) goto fail;
            break;
        case PAM_TEXT_INFO:
        case PAM_ERROR_MSG:
            if (messages[i]->msg) {
                int fd = open("/dev/tty", O_WRONLY | O_CLOEXEC | O_NOCTTY);
                if (fd >= 0) {
                    (void)write_all(fd, messages[i]->msg, strnlen(messages[i]->msg, 1024));
                    (void)write_all(fd, "\n", 1);
                    close(fd);
                }
            }
            break;
        default:
            goto fail;
        }
    }
    *responses = reply;
    return PAM_SUCCESS;
fail:
    for (int i = 0; i < count; i++) wipe_free(reply[i].resp);
    free(reply);
    return PAM_CONV_ERR;
}

static int create_private_config(char *dir, size_t dir_size) {
    const char *base = "/run";
    int n = snprintf(dir, dir_size, "%s/proxilens-pamtest.XXXXXX", base);
    if (n < 0 || (size_t)n >= dir_size || !mkdtemp(dir)) return -1;
    if (chmod(dir, 0700) != 0) return -1;
    char path[PATH_MAX];
    n = snprintf(path, sizeof(path), "%s/%s", dir, SERVICE_NAME);
    if (n < 0 || (size_t)n >= sizeof(path)) return -1;
    int fd = open(path, O_WRONLY | O_CREAT | O_EXCL | O_CLOEXEC | O_NOFOLLOW, 0600);
    if (fd < 0) return -1;
    size_t left = strlen(pam_conf);
    const char *p = pam_conf;
    while (left) {
        ssize_t w = write(fd, p, left);
        if (w < 0) { if (errno == EINTR) continue; close(fd); return -1; }
        p += (size_t)w;
        left -= (size_t)w;
    }
    int sync_status = fsync(fd);
    int close_status = close(fd);
    if (sync_status != 0 || close_status != 0) return -1;
    return 0;
}

static void remove_private_config(const char *dir) {
    char path[PATH_MAX];
    int n = snprintf(path, sizeof(path), "%s/%s", dir, SERVICE_NAME);
    if (n > 0 && (size_t)n < sizeof(path)) (void)unlink(path);
    (void)rmdir(dir);
}

static int original_sudo_uid(uid_t *out) {
    const char *uid_text = getenv("SUDO_UID");
    const char *sudo_user = getenv("SUDO_USER");
    if (!uid_text || !*uid_text || !sudo_user || !*sudo_user) return -1;

    errno = 0;
    char *end = NULL;
    unsigned long long parsed = strtoull(uid_text, &end, 10);
    if (errno != 0 || end == uid_text || *end != '\0' || parsed == 0 ||
        parsed > (unsigned long long)(uid_t)-1) return -1;

    *out = (uid_t)parsed;
    return 0;
}

int main(void) {
    /* sudo-rs, like sudo, runs the command with real/effective UID 0.
       The invoking user's identity is supplied in SUDO_UID/SUDO_USER. */
    if (getuid() != 0 || geteuid() != 0) {
        fprintf(stderr, "Bitte ausschließlich über sudo starten.\n");
        return 2;
    }
    uid_t invoking_uid = (uid_t)-1;
    if (original_sudo_uid(&invoking_uid) != 0) {
        fprintf(stderr, "Ursprüngliche Benutzeridentität fehlt; bitte über sudo starten.\n");
        return 2;
    }
    struct passwd *pw = getpwuid(invoking_uid);
    const char *sudo_user = getenv("SUDO_USER");
    if (!pw || !pw->pw_name || !*pw->pw_name || !sudo_user ||
        strcmp(pw->pw_name, sudo_user) != 0) {
        fprintf(stderr, "SUDO_UID und SUDO_USER stimmen nicht mit einem lokalen Konto überein.\n");
        return 2;
    }
    char username[256];
    if (strnlen(pw->pw_name, sizeof(username)) >= sizeof(username)) return 2;
    snprintf(username, sizeof(username), "%s", pw->pw_name);

    char confdir[PATH_MAX] = {0};
    if (create_private_config(confdir, sizeof(confdir)) != 0) {
        perror("private PAM-Testkonfiguration");
        if (confdir[0]) remove_private_config(confdir);
        return 2;
    }

    struct pam_conv conv = { conversation, NULL };
    pam_handle_t *pamh = NULL;
    int status = pam_start_confdir(SERVICE_NAME, username, &conv, confdir, &pamh);
    if (status == PAM_SUCCESS) status = pam_authenticate(pamh, 0);
    if (status == PAM_SUCCESS) status = pam_acct_mgmt(pamh, 0);
    if (pamh) {
        int end_status = pam_end(pamh, status);
        if (status == PAM_SUCCESS && end_status != PAM_SUCCESS) status = end_status;
    }
    remove_private_config(confdir);

    if (status == PAM_SUCCESS) {
        printf("PAM-Diagnose erfolgreich: lokales Konto wurde authentifiziert.\n");
        printf("Hinweis: Es wurde KEINE Systemaktion autorisiert und KEINE Freigabe erstellt.\n");
        return 0;
    }
    fprintf(stderr, "PAM-Diagnose fehlgeschlagen oder abgebrochen (PAM-Code %d). Keine Freigabe erteilt.\n", status);
    return 1;
}
