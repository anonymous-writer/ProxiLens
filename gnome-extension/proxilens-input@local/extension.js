import Clutter from 'gi://Clutter';
import Gio from 'gi://Gio';
import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';

const OBJECT_PATH = '/org/proxilens/Agent';
const INTERFACE = 'org.proxilens.Agent';

export default class ProxiLensInputTrigger extends Extension {
    enable() {
        this._sentForCurrentLock = false;
        this._eventId = global.stage.connect('captured-event', (_stage, event) => {
            if (!Main.screenShield.locked) {
                this._sentForCurrentLock = false;
                return Clutter.EVENT_PROPAGATE;
            }

            if (this._sentForCurrentLock)
                return Clutter.EVENT_PROPAGATE;

            const type = event.type();
            const isUserInput = type === Clutter.EventType.KEY_PRESS ||
                type === Clutter.EventType.BUTTON_PRESS ||
                type === Clutter.EventType.MOTION ||
                type === Clutter.EventType.TOUCH_BEGIN;

            if (!isUserInput)
                return Clutter.EVENT_PROPAGATE;

            // Send only the event category. Never inspect key values or pointer coordinates.
            try {
                Gio.DBus.session.emit_signal(
                    null,
                    OBJECT_PATH,
                    INTERFACE,
                    'UnlockRequested',
                    null
                );
                this._sentForCurrentLock = true;
            } catch (error) {
                console.error(`ProxiLens: could not notify agent: ${error}`);
            }
            return Clutter.EVENT_PROPAGATE;
        });
    }

    disable() {
        if (this._eventId) {
            global.stage.disconnect(this._eventId);
            this._eventId = 0;
        }
        this._sentForCurrentLock = false;
    }
}
