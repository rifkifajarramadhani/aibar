import Gio from 'gi://Gio';
import GLib from 'gi://GLib';

import {maxRecordBytes, parse, recordId} from './Records.mjs';

Gio._promisify(Gio.File.prototype, 'enumerate_children_async');
Gio._promisify(Gio.File.prototype, 'read_async');
Gio._promisify(Gio.FileEnumerator.prototype, 'next_files_async');
Gio._promisify(Gio.FileEnumerator.prototype, 'close_async');
Gio._promisify(Gio.InputStream.prototype, 'read_bytes_async');
Gio._promisify(Gio.InputStream.prototype, 'close_async');

const priority = GLib.PRIORITY_DEFAULT;
const decoder = new TextDecoder('utf-8', {fatal: true});

async function read(file, id, cancellable) {
    const stream = await file.read_async(priority, cancellable);
    try {
        const chunks = [];
        let length = 0;
        while (length <= maxRecordBytes) {
            const size = Math.min(65536, maxRecordBytes + 1 - length);
            const chunk = (await stream.read_bytes_async(size, priority, cancellable)).get_data();
            if (chunk.length === 0) {
                const bytes = new Uint8Array(length);
                let offset = 0;
                for (const part of chunks) {
                    bytes.set(part, offset);
                    offset += part.length;
                }
                return parse(decoder.decode(bytes), id);
            }
            chunks.push(chunk);
            length += chunk.length;
        }
        return null;
    } finally {
        await stream.close_async(priority, null);
    }
}

export async function scanRecords(directory, previous, cancellable) {
    let enumerator;
    try {
        enumerator = await directory.enumerate_children_async(
            'standard::name,standard::type,standard::size',
            Gio.FileQueryInfoFlags.NOFOLLOW_SYMLINKS, priority, cancellable);
    } catch (error) {
        if (error.matches(Gio.IOErrorEnum, Gio.IOErrorEnum.NOT_FOUND))
            return new Map();
        throw error;
    }

    const records = new Map();
    try {
        while (true) {
            const batch = await enumerator.next_files_async(32, priority, cancellable);
            if (batch.length === 0)
                break;

            for (const info of batch) {
                const id = recordId(info.get_name());
                if (id === null || info.get_file_type() !== Gio.FileType.REGULAR)
                    continue;

                let parsed = null;
                if (info.get_size() <= maxRecordBytes) {
                    try {
                        parsed = await read(enumerator.get_child(info), id, cancellable);
                    } catch (error) {
                        if (cancellable.is_cancelled())
                            throw error;
                    }
                }
                records.set(id, {
                    record: parsed ? parsed.record : previous.get(id)?.record ?? null,
                    unreadable: parsed === null,
                });
            }
        }
    } finally {
        await enumerator.close_async(priority, null);
    }
    return records;
}
