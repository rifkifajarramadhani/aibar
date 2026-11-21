import Atk from 'gi://Atk';
import Clutter from 'gi://Clutter';
import Gio from 'gi://Gio';
import GLib from 'gi://GLib';
import Pango from 'gi://Pango';
import St from 'gi://St';

import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import * as PanelMenu from 'resource:///org/gnome/shell/ui/panelMenu.js';
import * as PopupMenu from 'resource:///org/gnome/shell/ui/popupMenu.js';

import {countdown, history} from './Records.mjs';
import {scanRecords} from './reader.js';

const count = new Intl.NumberFormat(undefined, {maximumFractionDigits: 0});
const percent = new Intl.NumberFormat(undefined, {style: 'percent', maximumFractionDigits: 0});
const amount = new Intl.NumberFormat(undefined, {minimumFractionDigits: 2, maximumFractionDigits: 2});
const date = new Intl.DateTimeFormat(undefined, {weekday: 'short', month: 'short', day: 'numeric'});
const timestamp = new Intl.DateTimeFormat(undefined, {dateStyle: 'short', timeStyle: 'short'});

function label(text, styleClass = '') {
    const actor = new St.Label({text, style_class: styleClass});
    actor.clutter_text.set_use_markup(false);
    actor.clutter_text.set_line_wrap(true);
    actor.clutter_text.set_line_wrap_mode(Pango.WrapMode.WORD_CHAR);
    actor.clutter_text.set_ellipsize(Pango.EllipsizeMode.NONE);
    return actor;
}

function reading(parent, name, value) {
    const row = new St.BoxLayout({style_class: 'aibar-reading'});
    row.add_child(label(name));
    const display = new St.Bin({child: label(value), x_expand: true, x_align: Clutter.ActorAlign.END});
    row.add_child(display);
    parent.add_child(row);
}

function resetText(reset, now) {
    const duration = countdown(reset, now);
    if (duration.seconds === 0)
        return 'Reset time passed';
    const {days, hours, minutes, seconds} = duration;
    const parts = days ? [`${count.format(days)}d`, `${count.format(hours)}h`]
        : hours ? [`${count.format(hours)}h`, `${count.format(minutes)}m`]
        : [`${count.format(minutes)}m`, `${count.format(seconds % 60)}s`];
    return `Resets in ${parts.join(' ')}`;
}

export default class AibarExtension extends Extension {
    enable() {
        this._directory = Gio.File.new_for_path(GLib.build_filenamev([
            GLib.get_user_state_dir(), 'omarchy', 'agents', 'usage',
        ]));
        this._lifetime = new Gio.Cancellable();
        this._records = new Map();
        this._scanning = false;
        this._scanPending = false;
        this._loaded = false;
        this._readError = false;
        this._clock = 0;
        this._countdowns = [];

        const indicator = new PanelMenu.Button(0.5, 'AI usage');
        this._indicator = indicator;
        indicator.add_child(new St.Icon({
            icon_name: 'utilities-system-monitor-symbolic',
            style_class: 'system-status-icon',
        }));

        const section = new PopupMenu.PopupMenuSection();
        const content = new St.BoxLayout({orientation: Clutter.Orientation.VERTICAL, style_class: 'aibar-menu'});
        section.box.add_child(content);
        indicator.menu.addMenuItem(section);

        const header = new St.BoxLayout({style_class: 'aibar-header'});
        header.add_child(new St.Bin({
            child: label('AI usage', 'aibar-title'),
            x_expand: true,
            x_align: Clutter.ActorAlign.START,
        }));
        const reload = new St.Button({
            label: 'Reload records',
            style_class: 'button aibar-reload',
            can_focus: true,
        });
        reload.connect('clicked', () => this._scan());
        header.add_child(reload);
        content.add_child(header);

        this._status = label('', 'aibar-status');
        content.add_child(this._status);
        this._scroll = new St.ScrollView({
            style_class: 'aibar-scroll',
            hscrollbar_policy: St.PolicyType.NEVER,
            vscrollbar_policy: St.PolicyType.AUTOMATIC,
            overlay_scrollbars: false,
            can_focus: true,
            reactive: true,
            accessible_name: 'Provider usage',
            accessible_role: Atk.Role.SCROLL_PANE,
        });
        this._list = new St.BoxLayout({orientation: Clutter.Orientation.VERTICAL, style_class: 'aibar-providers'});
        this._scroll.set_child(this._list);
        const scrollKeys = new Clutter.KeyController();
        scrollKeys.connect('key-press', () => {
            const [, key] = scrollKeys.get_key();
            return this._scrollKey(key);
        });
        this._scroll.add_action(scrollKeys);
        content.add_child(this._scroll);

        indicator.menu.connect('open-state-changed', (_menu, open) => {
            if (open && this._lifetime) {
                this._render();
                this._scroll.vadjustment.value = 0;
                this._scan();
                this._clock = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 1000, () => {
                    this._tick();
                    return GLib.SOURCE_CONTINUE;
                });
            } else {
                this._stopClock();
            }
        });
        Main.panel.addToStatusArea(this.uuid, indicator, 0, 'right');

        this._poll = GLib.timeout_add_seconds(GLib.PRIORITY_DEFAULT, 30, () => {
            this._scan();
            return GLib.SOURCE_CONTINUE;
        });
        this._scan();
    }

    disable() {
        const lifetime = this._lifetime;
        this._lifetime = null;
        lifetime.cancel();
        GLib.Source.remove(this._poll);
        this._stopClock();
        this._indicator.destroy();
        this._indicator = null;
        this._directory = null;
        this._records = null;
        this._status = null;
        this._scroll = null;
        this._list = null;
        this._countdowns = [];
    }

    _stopClock() {
        if (this._clock)
            GLib.Source.remove(this._clock);
        this._clock = 0;
    }

    async _scan() {
        if (this._scanning) {
            this._scanPending = true;
            return;
        }

        const lifetime = this._lifetime;
        this._scanning = true;
        try {
            do {
                this._scanPending = false;
                try {
                    const records = await scanRecords(this._directory, this._records, lifetime);
                    if (lifetime.is_cancelled())
                        return;
                    this._records = records;
                    this._readError = false;
                } catch (error) {
                    if (lifetime.is_cancelled())
                        return;
                    this._readError = true;
                }
                this._loaded = true;
                if (this._indicator.menu.isOpen)
                    this._render();
            } while (this._scanPending);
        } finally {
            if (!lifetime.is_cancelled())
                this._scanning = false;
        }
    }

    _scrollKey(key) {
        const adjustment = this._scroll.vadjustment;
        switch (key) {
        case Clutter.KEY_Down:
            adjustment.value += adjustment.step_increment;
            break;
        case Clutter.KEY_Up:
            if (adjustment.value <= adjustment.lower)
                return Clutter.EVENT_PROPAGATE;
            adjustment.value -= adjustment.step_increment;
            break;
        case Clutter.KEY_Page_Down:
            adjustment.value += adjustment.page_increment;
            break;
        case Clutter.KEY_Page_Up:
            adjustment.value -= adjustment.page_increment;
            break;
        case Clutter.KEY_Home:
            adjustment.value = adjustment.lower;
            break;
        case Clutter.KEY_End:
            adjustment.value = adjustment.upper - adjustment.page_size;
            break;
        default:
            return Clutter.EVENT_PROPAGATE;
        }
        return Clutter.EVENT_STOP;
    }

    _tick() {
        const now = new Date();
        if (this._day !== now.toDateString()) {
            this._render(now);
            return;
        }
        for (const {actor, reset} of this._countdowns) {
            const text = resetText(reset, +now);
            if (actor.text !== text)
                actor.text = text;
        }
    }

    _render(now = new Date()) {
        this._day = now.toDateString();
        this._countdowns = [];
        this._list.destroy_all_children();
        const entries = [...this._records.values()];
        const providers = entries.filter(entry => entry.record)
            .sort((a, b) => a.record.id.localeCompare(b.record.id));
        let status = '';
        if (this._readError)
            status = 'Could not read the usage records folder.';
        else if (entries.some(entry => entry.unreadable))
            status = 'Some usage records could not be read.';
        else if (!this._loaded)
            status = 'Reading usage records…';
        else if (providers.length === 0)
            status = 'No usage records found. Set up a usage record producer, then reload.';
        this._status.text = status;
        this._status.visible = status !== '';
        this._scroll.visible = providers.length > 0;

        for (const {record, unreadable} of providers)
            this._provider(record, unreadable, now);
    }

    _provider(record, unreadable, now) {
        const section = new St.BoxLayout({orientation: Clutter.Orientation.VERTICAL, style_class: 'aibar-provider'});
        this._list.add_child(section);
        const title = label(record.name, 'aibar-title');
        title.set_accessible_role(Atk.Role.HEADING);
        section.add_child(title);
        if (unreadable)
            section.add_child(label('Showing last readable record', 'aibar-status'));
        this._details(section, record, record.updated, +now);

        for (const account of record.accounts) {
            const details = new St.BoxLayout({orientation: Clutter.Orientation.VERTICAL, style_class: 'aibar-details'});
            details.add_child(label(account.name, 'aibar-subtitle'));
            if (account.active)
                details.add_child(label('Active account'));
            if (account.email)
                details.add_child(label(account.email));
            this._details(details, account, null, +now);
            section.add_child(details);
        }

        const days = history(record, +now);
        const updatedToday = record.updated !== null && new Date(record.updated).toDateString() === this._day;
        const tokens = days[days.length - 1].tokens;
        const prompts = updatedToday ? record.todayPrompts : null;
        const sessions = updatedToday ? record.todaySessions : null;
        if (record.hasActivity || tokens !== null || prompts !== null || sessions !== null) {
            const today = new St.BoxLayout({orientation: Clutter.Orientation.VERTICAL, style_class: 'aibar-details'});
            today.add_child(label('Today', 'aibar-subtitle'));
            reading(today, 'Tokens', tokens === null ? '—' : count.format(tokens));
            if (prompts !== null)
                reading(today, 'Prompts', count.format(prompts));
            if (sessions !== null)
                reading(today, 'Sessions', count.format(sessions));
            section.add_child(today);
        }
        if (record.hasActivity || days.some(day => day.tokens !== null)) {
            const recent = new St.BoxLayout({orientation: Clutter.Orientation.VERTICAL, style_class: 'aibar-details'});
            recent.add_child(label('Last 7 days · Tokens', 'aibar-subtitle'));
            for (const day of days)
                reading(recent, date.format(day.date), day.tokens === null ? '—' : count.format(day.tokens));
            section.add_child(recent);
        }
    }

    _details(parent, data, updated, now) {
        if (data.plan)
            reading(parent, 'Plan', data.plan);
        if (data.stale) {
            const recorded = data.fetched ?? updated;
            const text = recorded === null ? 'Cached limits' : `Cached limits · ${timestamp.format(recorded)}`;
            parent.add_child(label(text, 'aibar-status'));
        }
        for (const limit of data.limits) {
            reading(parent, limit.label || 'Limit', percent.format(limit.percent));
            if (limit.reset !== null) {
                const actor = label(resetText(limit.reset, now), 'aibar-status');
                parent.add_child(actor);
                this._countdowns.push({actor, reset: limit.reset});
            }
        }
        if (data.balance) {
            const {remaining, funded, spent, currency, estimated} = data.balance;
            const money = value => `${amount.format(value)} ${currency}`;
            reading(parent, estimated ? 'Estimated balance' : 'Balance', money(remaining));
            if (funded !== null)
                reading(parent, 'Funded', money(funded));
            if (spent !== null)
                reading(parent, 'Spent', money(spent));
        }
        if (data.resetCredits !== null)
            reading(parent, 'Reset credits', count.format(data.resetCredits));
        if (data.status)
            parent.add_child(label(data.status));
        if (data.help)
            parent.add_child(label(data.help));
    }
}
