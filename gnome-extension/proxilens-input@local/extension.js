import Clutter from 'gi://Clutter';
import Gio from 'gi://Gio';
import GLib from 'gi://GLib';
import St from 'gi://St';
import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';

const OBJECT_PATH = '/org/proxilens/Agent';
const INTERFACE = 'org.proxilens.Agent';
const SIGNAL = 'DuoUnlockRequested';
const LOCK_STATE_SIGNAL = 'LockStateChanged';
const AUTH_STATE_SIGNAL = 'DuoAuthStateChanged';

export default class ProxiLensLockButton extends Extension {
    enable() {
        this._sessionModeId = Main.sessionMode.connect('updated', () => this._sync());
        this._authStateSubscription = Gio.DBus.session.signal_subscribe(
            null, INTERFACE, AUTH_STATE_SIGNAL, OBJECT_PATH, null,
            Gio.DBusSignalFlags.NONE,
            (_connection, _sender, _path, _iface, _signal, parameters) => {
                const [state] = parameters.deep_unpack();
                this._showAuthState(state);
            }
        );
        this._sync();
    }

    _showAuthState(state) {
        if (!this._button)
            return;
        const labels = {
            locked: 'Mit Duo entsperren',
            unlocked: 'Mit Duo entsperren',
            pending: 'Duo-Push läuft …',
            success: 'Duo bestätigt – entsperre …',
            'unlock-requested': 'Entsperrung wird ausgeführt …',
            failed: 'Duo fehlgeschlagen – erneut versuchen',
        };
        this._button.label = labels[state] ?? 'Mit Duo entsperren';
        this._button.reactive = state === 'failed' || state === 'locked' || state === 'unlocked';
        if (state === 'failed')
            this._requestPending = false;
    }

    _sync() {
        const inUnlockDialog = Main.sessionMode.currentMode === 'unlock-dialog';
        this._reportLockState(inUnlockDialog);
        if (inUnlockDialog) {
            this._addButton();
        } else {
            this._removeButton();
        }
    }

    // Report GNOME's actual session mode to the user agent. This is a status
    // hint only; the agent still verifies logind LockedHint before unlocking.
    _reportLockState(locked) {
        if (this._lastReportedLockState === locked)
            return;
        this._lastReportedLockState = locked;
        try {
            Gio.DBus.session.emit_signal(
                null,
                OBJECT_PATH,
                INTERFACE,
                LOCK_STATE_SIGNAL,
                new GLib.Variant('(b)', [locked])
            );
        } catch (error) {
            console.error(`ProxiLens: Sperrstatus konnte nicht gemeldet werden: ${error}`);
        }
    }

    _addButton() {
        const dialog = Main.screenShield?._dialog;
        const clock = dialog?._clock;
        if (!clock || this._button)
            return;

        this._button = new St.Button({
            label: 'Mit Duo entsperren',
            style_class: 'button suggested-action proxilens-duo-unlock',
            can_focus: true,
            reactive: true,
            track_hover: true,
            x_align: Clutter.ActorAlign.CENTER,
            y_align: Clutter.ActorAlign.CENTER,
        });
        this._button.connect('clicked', () => this._requestDuoUnlock());
        if (clock._date)
            clock.insert_child_above(this._button, clock._date);
        else
            clock.add_child(this._button);
        this._clock = clock;
    }

    _requestDuoUnlock() {
        if (!this._button || this._requestPending)
            return;
        this._requestPending = true;
        this._button.reactive = false;
        try {
            Gio.DBus.session.emit_signal(
                null,
                OBJECT_PATH,
                INTERFACE,
                SIGNAL,
                null
            );
        } catch (error) {
            console.error(`ProxiLens: Duo-Anfrage konnte nicht gesendet werden: ${error}`);
            this._button.reactive = true;
            this._requestPending = false;
        }
    }

    _removeButton() {
        if (this._button) {
            this._button.destroy();
            this._button = null;
        }
        this._clock = null;
        this._requestPending = false;
    }

    disable() {
        if (this._sessionModeId) {
            Main.sessionMode.disconnect(this._sessionModeId);
            this._sessionModeId = 0;
        }
        if (this._authStateSubscription) {
            Gio.DBus.session.signal_unsubscribe(this._authStateSubscription);
            this._authStateSubscription = 0;
        }
        this._removeButton();
        this._reportLockState(false);
        this._lastReportedLockState = undefined;
    }
}
