# ProxiLens 2.1.0 Beta 10.1 – Authentication workflow preview

## Added

- Explicit cancellation of an in-flight Duo Push request. Cancellation is passed
  through the request context; an aborted request does not enable the test grant.
- Clearer UI states for Duo start, waiting/progress, success, rejection, timeout,
  and cancellation.
- Explicitly labelled local 15-minute simulation grant, with a revoke control.
- A visible Ubuntu-password/PAM choice explaining that the system-password path
  is not active yet. This build never collects, stores, or validates the Ubuntu
  password in the desktop application.

## Safety boundary

The grant broker is still a per-user development simulation. The UI does not
create a privileged approval, and no PAM, Polkit, sudoers, or system service
configuration is installed or modified. The action-scoped `policygrant` model
is not yet connected to the desktop flow or a privileged authorization daemon.
