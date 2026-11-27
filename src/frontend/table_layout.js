// Column layouts (order, visibility and user-set widths) for the GUI tables.
// Layouts are saved per table in settings.json under gui.table_layouts.

// The row-number column needs a field so it can be moved and hidden like the others.
export const ROW_NUMBER_FIELD = "row_number";

function savedColumns(layout) {
    if (!layout || !Array.isArray(layout.columns)) {
        return [];
    }
    return layout.columns.filter(column => column && typeof column.field === "string" && column.field !== "");
}

// Returns column definitions in the saved order with saved visibility and widths.
// Columns missing from the layout (for example ones added in a newer version) follow
// in their default order with their default visibility. Saved entries for columns that
// no longer exist are ignored. Fields in hiddenFields stay hidden whatever is saved;
// an application setting controls them.
//
// Tabulator exports a column with download: true even while it is hidden. Only a locked
// field keeps it, and only while the user's own layout still shows that column: Version,
// hidden by "hide version string", stays in exports unless the user hid it from the menu
// earlier. Every other column is exported exactly when it is visible, so a column the user
// hides from the menu is left out. If nothing would be visible (every other
// column hidden, then the one left locked by a setting), the default columns are shown,
// so there is still a header to right-click; the saved layout is kept until the user
// changes it.
export function applyTableLayout(columns, layout, hiddenFields = []) {
    const locked = new Set(hiddenFields);
    const saved = new Map();
    savedColumns(layout).forEach((column, index) => {
        if (!saved.has(column.field)) {
            saved.set(column.field, {column, index});
        }
    });

    const known = columns.filter(column => saved.has(column.field))
        .sort((a, b) => saved.get(a.field).index - saved.get(b.field).index);
    const added = columns.filter(column => !saved.has(column.field));

    const result = known.concat(added).map(definition => {
        const column = Object.assign({}, definition);
        const entry = saved.get(definition.field);
        if (entry) {
            column.visible = entry.column.visible !== false;
            if (Number.isFinite(entry.column.width) && entry.column.width > 0) {
                column.width = entry.column.width;
            }
        }
        if (column.download === true) {
            if (!locked.has(definition.field)) {
                delete column.download;
            } else if (entry && entry.column.visible === false) {
                column.download = false;
            }
        }
        if (locked.has(definition.field)) {
            column.visible = false;
        }
        return column;
    });

    if (!result.some(column => column.visible !== false)) {
        const defaults = columns.filter(column => column.visible !== false && !locked.has(column.field)).map(column => column.field);
        const fallback = new Set(defaults.length ? defaults : result.filter(column => !locked.has(column.field)).slice(0, 1).map(column => column.field));
        for (const column of result) {
            if (fallback.has(column.field)) {
                column.visible = true;
            }
        }
    }
    return result;
}

// Fields whose width the user chose: saved widths plus columns resized since.
export function savedWidthFields(layout) {
    return new Set(savedColumns(layout).filter(column => Number.isFinite(column.width) && column.width > 0).map(column => column.field));
}

// Builds the layout to save from Tabulator column components. A width is only saved
// for columns the user resized, so other columns keep fitting the window. Locked
// fields keep their previously saved visibility, because the setting that hides
// them is not the user's choice for this table.
export function captureTableLayout(columns, resizedFields, previousLayout, hiddenFields = []) {
    const locked = new Set(hiddenFields);
    const previous = new Map(savedColumns(previousLayout).map(column => [column.field, column]));
    const result = [];
    for (const column of columns) {
        const field = column.getField();
        if (!field) {
            continue;
        }
        const entry = {field, visible: column.isVisible()};
        if (locked.has(field)) {
            entry.visible = previous.has(field) ? previous.get(field).visible !== false : true;
        }
        if (resizedFields.has(field)) {
            const width = Math.round(column.getWidth());
            if (width > 0) {
                entry.width = width;
            }
        }
        result.push(entry);
    }
    return {columns: result};
}

// Returns save(key, layout), which sends each layout change straight away so it is not
// lost if the window closes right after (closing does not run the page's unload handlers
// in the desktop app). Saves for one table go one at a time: while one is being sent, only
// the newest waiting layout is kept and sent when it finishes, so an older layout can never
// land after a newer one, whatever order the backend handles calls in. send(key, layout)
// may return a promise; an error, thrown or rejected, is logged and does not stop later
// saves. send should report its own errors without waiting on the user, because the next
// save waits for it.
export function serialSaver(send) {
    const tables = new Map();
    const next = (key, entry) => {
        if (!entry.waiting) {
            entry.sending = false;
            return;
        }
        const layout = entry.layout;
        entry.waiting = false;
        entry.layout = undefined;
        entry.sending = true;
        let result;
        try {
            result = send(key, layout);
        } catch (error) {
            result = Promise.reject(error);
        }
        Promise.resolve(result)
            .catch(error => console.error("table layout save failed", key, error))
            .then(() => next(key, entry));
    };
    return (key, layout) => {
        let entry = tables.get(key);
        if (!entry) {
            entry = {sending: false, waiting: false, layout: undefined};
            tables.set(key, entry);
        }
        entry.layout = layout;
        entry.waiting = true;
        if (!entry.sending) {
            next(key, entry);
        }
    };
}

export function escapeHtml(value) {
    return String(value).replace(/[&<>"']/g, character => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;"
    })[character]);
}

export function columnLabel(column, labels) {
    const field = column.getField();
    const definition = column.getDefinition ? column.getDefinition() : {};
    return (labels && labels[field]) || definition.title || field;
}

// Header context-menu items: one entry per column to show or hide it, then a reset.
// The last visible column cannot be hidden. Locked fields are left out.
export function columnMenuItems(columns, {labels = {}, hiddenFields = [], onToggle, onReset}) {
    const locked = new Set(hiddenFields);
    const toggleable = columns.filter(column => column.getField() && !locked.has(column.getField()));
    const visibleCount = toggleable.filter(column => column.isVisible()).length;

    const items = toggleable.map(column => {
        const visible = column.isVisible();
        return {
            label: '<span class="slm-column-menu-check" aria-hidden="true">' + (visible ? "&#10003;" : "") + "</span>" +
                escapeHtml(columnLabel(column, labels)),
            disabled: visible && visibleCount <= 1,
            action: () => onToggle(column, !visible)
        };
    });
    items.push({separator: true});
    items.push({label: "Reset columns", action: () => onReset()});
    return items;
}

// Header content padding (8px each side in the Bootstrap Tabulator theme), the title
// padding a sortable header keeps clear for its arrow (set in app.css), and a little
// slack for borders and rounding.
const HEADER_PADDING = 16;
const HEADER_SORT_ARROW = 16;
const HEADER_SLACK = 4;

// Smallest column width that keeps a header title on one line, so every table's header
// stays one title line plus its filter box tall however many columns are shown; columns
// that no longer fit the window scroll sideways instead.
export function headerMinWidth(title, sortable, measureText) {
    const text = String(title ?? "").trim().replace(/\s+/g, " ");
    if (!text) {
        return 0;
    }
    return Math.ceil(measureText(text)) + HEADER_PADDING + HEADER_SLACK + (sortable ? HEADER_SORT_ARROW : 0);
}

// Raises each titled column's minWidth to fit its title, including columns with a fixed or
// saved width (Tabulator widens those to their minimum). tableHeaderSort is the table-wide
// headerSort option (Tabulator defaults to true).
export function withHeaderMinWidths(columns, measureText, tableHeaderSort = true) {
    return columns.map(column => {
        if (!column.title) {
            return column;
        }
        const sortable = (column.headerSort ?? tableHeaderSort) !== false;
        const minWidth = headerMinWidth(column.title, sortable, measureText);
        return minWidth > (column.minWidth || 0) ? Object.assign({}, column, {minWidth}) : column;
    });
}

// Cell content padding (8px each side in the Bootstrap Tabulator theme) plus slack for the
// cell border and rounding.
const CELL_PADDING = 16;
const CELL_SLACK = 4;

// Smallest column width that shows the widest of the given values on one line, so IDs
// never wrap ("0100E050113500 / 00"). Returns 0 when there is nothing to measure.
export function contentMinWidth(values, measureText) {
    let widest = 0;
    for (const value of new Set(values)) {
        if (typeof value === "string" && value !== "") {
            widest = Math.max(widest, measureText(value));
        }
    }
    return widest ? Math.ceil(widest) + CELL_PADDING + CELL_SLACK : 0;
}

// Smallest column width that fits the longest word of the given values (text, or lists
// shown joined with commas), so text wraps only between words. Capped at maxWidth so one
// very long word cannot take over the table; such a word still breaks.
export function longestWordWidth(values, measureText, maxWidth = 150) {
    const words = new Set();
    for (const value of new Set(values)) {
        for (const word of joinList(value).split(/\s+/)) {
            if (word) {
                words.add(word);
            }
        }
    }
    let widest = 0;
    for (const word of words) {
        widest = Math.max(widest, measureText(word));
    }
    return widest ? Math.min(maxWidth, Math.ceil(widest) + CELL_PADDING + CELL_SLACK) : 0;
}

// Narrowest column width at which every value (text, or a list shown joined with commas)
// wraps to at most maxLines lines, so no row grows taller than that; a column that needs
// more room is widened and the table scrolls sideways instead. widthOf(text) is a text's
// width on one line; countLines(contentWidth, texts) lays the texts out that wide, as the
// table does, and returns how many lines each takes. Returns 0 when there is nothing to
// measure.
export function linesWidth(values, maxLines, widthOf, countLines) {
    const texts = [];
    for (const text of new Set(Array.from(values, joinList))) {
        if (text) {
            texts.push({text, width: widthOf(text)});
        }
    }
    if (!texts.length) {
        return 0;
    }
    // Texts that fail at a width, among those that would wrap there (the slack covers
    // rounding differences between widthOf and the real layout).
    const failing = (width, candidates) => {
        const wrapping = candidates.filter(entry => entry.width > width - CELL_SLACK);
        if (!wrapping.length) {
            return [];
        }
        const lines = countLines(width, wrapping.map(entry => entry.text));
        return wrapping.filter((entry, index) => lines[index] > maxLines);
    };
    const widest = Math.max(...texts.map(entry => entry.width));
    // Narrower than an even share of the widest text cannot work; the widest text on one
    // line always does.
    let low = Math.max(1, Math.floor(widest / maxLines));
    let high = Math.ceil(widest) + CELL_SLACK;
    // A text that fits at some width fits at every wider one, so after one layout of all
    // texts at the lowest width only the ones that did not fit need searching.
    let remaining = failing(low, texts);
    if (!remaining.length) {
        return low + CELL_PADDING + CELL_SLACK;
    }
    low += 1;
    while (low < high) {
        const middle = Math.floor((low + high) / 2);
        const failed = failing(middle, remaining);
        if (failed.length) {
            remaining = failed;
            low = middle + 1;
        } else {
            high = middle;
        }
    }
    return high + CELL_PADDING + CELL_SLACK;
}

// Width that fits any number of up to `digits` digits, whichever digits they are.
export function digitsWidth(digits, measureText) {
    const widest = Math.max(...Array.from("0123456789", digit => measureText(digit)));
    return Math.ceil(widest * digits) + CELL_PADDING + CELL_SLACK;
}

// Tabulator 4.7's fitColumns layout counts its rounding gap twice once a column is held
// at its minimum width, so the table ends up a few pixels wider than its container and
// shows a horizontal scrollbar. Given the laid-out columns in display order, returns
// widths with any overflow taken from flexible columns (last first), never below their
// minimum width. Columns with a set width are left alone.
export function trimOverflow(columns, available) {
    const widths = columns.map(column => column.width);
    let overflow = widths.reduce((sum, width) => sum + width, 0) - available;
    for (let i = columns.length - 1; i >= 0 && overflow > 0; i--) {
        if (!columns[i].flexible) {
            continue;
        }
        const cut = Math.min(overflow, widths[i] - (columns[i].minWidth || 0));
        if (cut > 0) {
            widths[i] -= cut;
            overflow -= cut;
        }
    }
    return widths;
}

// Keeps a menu opened at the pointer inside the viewport. Tabulator flips a menu that
// would run past the bottom upward from the pointer, which can push its first items
// above the window; instead open it at the pointer and shift it up or left only as far
// as needed. A menu taller than the window is capped by CSS and scrolls.
export function menuPosition(pointer, size, viewport, margin = 8) {
    const clamp = (value, available) => Math.max(margin, Math.min(value, available - margin));
    return {
        top: clamp(pointer.y, viewport.height - size.height),
        left: clamp(pointer.x, viewport.width - size.width)
    };
}

// Joins list metadata (genres, languages, ...) for display and export, dropping
// blanks and repeats. Strings pass through, so it is safe to apply more than once.
export function joinList(value) {
    if (Array.isArray(value)) {
        const seen = new Set();
        for (const item of value) {
            const text = typeof item === "string" ? item.trim() : "";
            if (text) {
                seen.add(text);
            }
        }
        return Array.from(seen).join(", ");
    }
    return typeof value === "string" ? value : "";
}

const SIZE_UNITS = ["B", "KB", "MB", "GB", "TB"];

// Formats a byte count for display; zero, negative or missing sizes are blank.
export function formatFileSize(bytes) {
    if (!Number.isFinite(bytes) || bytes <= 0) {
        return "";
    }
    let value = bytes;
    let unit = 0;
    while (value >= 1024 && unit < SIZE_UNITS.length - 1) {
        value /= 1024;
        unit++;
    }
    return (unit === 0 ? String(value) : value.toFixed(value >= 100 ? 0 : 1)) + " " + SIZE_UNITS[unit];
}
