import Clutter from 'gi://Clutter';
import Gio from 'gi://Gio';
import GLib from 'gi://GLib';
import St from 'gi://St';

import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import {disableHelperAutoExit, sleep} from 'resource:///org/gnome/shell/ui/scripting.js';

const uuid = 'aibar@rifkifajarramadhani.github.io';

function assert(condition, message) {
    if (!condition)
        throw new Error(message);
}

async function until(predicate, message, timeout = 5000) {
    const deadline = GLib.get_monotonic_time() + timeout * 1000;
    while (!predicate()) {
        assert(GLib.get_monotonic_time() < deadline, message);
        await sleep(50);
    }
}

function find(actor, predicate) {
    if (actor.visible && predicate(actor))
        return actor;
    if (actor.visible) {
        for (const child of actor.get_children()) {
            const match = find(child, predicate);
            if (match)
                return match;
        }
    }
    return null;
}

export async function run() {
    await disableHelperAutoExit();
    const state = GLib.getenv('XDG_STATE_HOME');
    assert(state && !GLib.file_test(state, GLib.FileTest.EXISTS),
        'Set XDG_STATE_HOME to a fresh, nonexistent directory');
    const agentDir = GLib.build_filenamev([state, 'omarchy', 'agents']);
    const directory = GLib.build_filenamev([agentDir, 'usage']);
    const path = id => GLib.build_filenamev([directory, `${id}.json`]);
    const indicator = () => Main.panel.statusArea[uuid];
    const menu = () => indicator().menu;
    const findText = text => find(menu().actor, actor => actor instanceof St.Label && actor.text === text);
    const clock = () => find(menu().actor, actor =>
        actor instanceof St.Label && actor.text.startsWith('Resets in '))?.text;
    const keyboard = global.stage.context.get_backend().get_default_seat()
        .create_virtual_device(Clutter.InputDeviceType.KEYBOARD_DEVICE);
    function key(symbol) {
        keyboard.notify_keyval(GLib.get_monotonic_time(), symbol, Clutter.KeyState.PRESSED);
        keyboard.notify_keyval(GLib.get_monotonic_time(), symbol, Clutter.KeyState.RELEASED);
    }
    async function open() {
        indicator().grab_key_focus();
        key(Clutter.KEY_Down);
        await until(() => menu().isOpen, 'Down did not open the panel menu');
    }

    await until(indicator, 'Extension did not load');
    await open();
    await until(() => findText('No usage records found. Set up a usage record producer, then reload.'),
        'Missing directory did not show the empty state');
    const reload = find(menu().actor, actor => actor instanceof St.Button && actor.label === 'Reload records');
    assert(global.stage.key_focus === reload, 'Reload records did not get initial keyboard focus');

    GLib.mkdir_with_parents(directory, 0o700);
    const now = new Date();
    const longLimit = 'Weekly allowance for the shared account with additional capacity during the current reporting period';
    const prompts = 123456789012345;
    const promptCount = new Intl.NumberFormat().format(prompts);
    const record = {
        schemaVersion: 1, id: 'demo', ready: true, name: '<b>Demo</b>', tierLabel: 'Trial',
        updatedAt: now.toISOString(),
        limits: [
            {label: '5h window', percent: 0.37, resetsAt: +now + 120000},
            {label: longLimit, percent: 0.91},
        ],
        balance: {remaining: 8.5, funded: 10, spent: 1.5, currency: 'USD'},
        resetCredits: {available: 0},
        accounts: [{id: 'work', label: 'Work', email: 'work@example.invalid', active: true}],
        todayTotalTokens: 0, todayPrompts: prompts, todaySessions: 3,
    };
    function replace(text) {
        const staging = GLib.build_filenamev([directory, 'demo.tmp']);
        GLib.file_set_contents(staging, text);
        Gio.File.new_for_path(staging).move(Gio.File.new_for_path(path('demo')),
            Gio.FileCopyFlags.OVERWRITE, null, null);
    }
    replace(JSON.stringify(record));
    key(Clutter.KEY_Return);
    await until(() => findText('37%'), 'Keyboard reload did not find a newly created record');
    assert(menu().isOpen, 'Reload closed the menu');
    for (const expected of ['<b>Demo</b>', 'Trial', '8.50 USD', 'Reset credits', 'Work',
        'work@example.invalid', 'Active account', 'Today', promptCount, 'Last 7 days · Tokens', '0', '—'])
        assert(findText(expected), `Missing reading ${expected}`);
    assert(!findText('<b>Demo</b>').clutter_text.use_markup, 'Provider heading uses markup');
    const limitLabel = findText(longLimit);
    const limitValue = findText('91%');
    await until(() => limitLabel.clutter_text.get_layout().get_line_count() > 1,
        'Long quota label did not wrap');
    const [labelX] = limitLabel.get_transformed_position();
    const [valueX] = limitValue.get_transformed_position();
    assert(labelX + limitLabel.width <= valueX,
        'Long quota label overlaps its percentage');
    const shortValue = findText('37%');
    const [shortValueX] = shortValue.get_transformed_position();
    assert(Math.abs(valueX + limitValue.width - shortValueX - shortValue.width) <= 1,
        'Quota percentages do not share a right edge');
    const oldClock = clock();
    assert(oldClock, 'Missing reset countdown');
    await until(() => clock() !== oldClock, 'Countdown did not tick');

    const scroll = find(menu().actor, actor => actor instanceof St.ScrollView);
    const adjustment = scroll.vadjustment;
    await until(() => adjustment.upper > adjustment.page_size, 'Provider list is not scrollable');
    key(Clutter.KEY_Tab);
    await until(() => global.stage.key_focus === scroll, 'Tab did not focus the provider list');
    key(Clutter.KEY_End);
    await until(() => adjustment.value > 0, 'End did not scroll the provider list');
    key(Clutter.KEY_Home);
    await until(() => adjustment.value === 0, 'Home did not return to the top');

    key(Clutter.KEY_Escape);
    await until(() => !menu().isOpen, 'Escape did not close the menu');
    const extension = Main.extensionManager.lookup(uuid).stateObj;
    assert(extension._clock === 0, 'Countdown timer kept running with the menu closed');

    replace(JSON.stringify({...record, name: 'Replacement', limits: [{label: '5h window', percent: 0.62}]}));
    await open();
    await until(() => findText('62%'), 'Opening did not scan the atomic replacement');

    replace('{"schemaVersion":1,');
    key(Clutter.KEY_Return);
    await until(() => findText('Some usage records could not be read.'), 'Malformed record was not reported');
    assert(findText('Replacement') && findText('62%'), 'Malformed replacement lost the cached reading');

    Gio.File.new_for_path(path('demo')).delete(null);
    key(Clutter.KEY_Return);
    await until(() => findText('No usage records found. Set up a usage record producer, then reload.'),
        'Deleted record did not clear the list');
    assert(!findText('Replacement'), 'Deleted provider is still displayed');

    const folder = Gio.File.new_for_path(directory);
    folder.delete(null);
    GLib.file_set_contents(directory, 'not a directory');
    key(Clutter.KEY_Return);
    await until(() => findText('Could not read the usage records folder.'), 'Folder read error was not reported');
    folder.delete(null);
    GLib.mkdir_with_parents(directory, 0o700);
    replace(JSON.stringify({...record, name: 'Polled provider'}));
    await until(() => findText('Polled provider'), 'The 30-second poll did not find the record', 35000);

    // Disable with a read in progress and start a new lifetime immediately.
    extension._scan();
    const oldLifetime = extension._lifetime;
    Main.extensionManager.disableExtension(uuid);
    await until(() => !indicator(), 'Disable left a panel indicator behind');
    assert(oldLifetime.is_cancelled() && extension._clock === 0, 'Disable did not stop pending work');
    Main.extensionManager.enableExtension(uuid);
    await until(indicator, 'Re-enable did not restore the panel indicator');
    await until(() => extension._records.has('demo'), 'Re-enable did not scan at startup');
    await open();
    await until(() => findText('Polled provider'), 'Re-enabled menu did not show records');

    print('Verified GNOME panel keyboard access, scrolling, countdowns, record reads, and extension lifecycle.');
}
