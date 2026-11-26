# Beta 9.2 changes

- Hardened probe request framing: one bounded newline-terminated JSON frame;
  trailing frames/data are rejected.
- Added process start-time checks around session lookup to detect PID reuse.
- Changed cgroup session matching to exact path-component comparison.
- Added tests for trailing data, exact session scope matching, and process
  start-time retrieval.
- Added PAM development design and safety gates only; no loadable PAM module,
  PAM stack file, or system authentication change is included.
