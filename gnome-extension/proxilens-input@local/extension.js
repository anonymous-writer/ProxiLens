import Gio from 'gi://Gio';
import St from 'gi://St';
import Clutter from 'gi://Clutter';
import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';

const OBJECT_PATH = '/org/proxilens/Agent';
const INTERFACE = 'org.proxilens.Agent';
const SIGNAL = 'DuoUnlockRequested';

export default class ProxiLensLockButton extends Extension {
    enable() {
        this._sessionModeId = Main.sessionMode.connect('updated', () => this._sync());
        this._sync();
    }

    _sync() {
        const inUnlockDialog = Main.sessionMode.currentMode === 'unlock-dialog';
        if (inUnlockDialog) {
            this._addButton();
        } else {
            this._removeButton();
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
        this._removeButton();
    }
}
