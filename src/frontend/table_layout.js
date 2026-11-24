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

    return known.concat(added).map(definition => {
        const result = Object.assign({}, definition);
        const entry = saved.get(definition.field);
        if (entry) {
            result.visible = entry.column.visible !== false;
            if (Number.isFinite(entry.column.width) && entry.column.width > 0) {
                result.width = entry.column.width;
            }
        }
        if (locked.has(definition.field)) {
            result.visible = false;
        }
        return result;
    });
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
