// Column definitions shared by the GUI tables, including the optional catalog
// columns (issue #188) that stay hidden until the user shows them.
import { ROW_NUMBER_FIELD, escapeHtml, formatFileSize, joinList } from './table_layout.js'

// Catalog fields as sent in title attributes (Missing updates, Missing DLC) ...
export const CATALOG_FIELDS = {
    publisher: "publisher",
    genres: "category",
    languages: "languages",
    players: "numberOfPlayers",
    rating: "rating",
    ratingContent: "ratingContent"
};

// ... and as sent in Library and Missing games rows.
export const ROW_METADATA_FIELDS = {
    publisher: "publisher",
    genres: "genres",
    languages: "languages",
    players: "players",
    rating: "age_rating",
    ratingContent: "rating_content"
};

// Layout for tables whose columns share the window width in proportion, so nothing
// scrolls sideways however many columns are shown; text wraps instead (see app.css).
// Classic rendering re-measures wrapped rows after columns are sized and on window
// resize; the virtual renderer keeps stale row heights. Pair it with a tableBuilt
// callback that re-fits when the table's own width changes.
export const FIT_TO_WINDOW = Object.freeze({layout: "fitColumns", columnMinWidth: 24, virtualDom: false});

export function rowNumberColumn() {
    return {formatter: "rownum", field: ROW_NUMBER_FIELD, headerSort: false, download: false};
}

export function textColumn(title, field) {
    return {title, field, visible: false, headerFilter: "input", formatter: "textarea"};
}

// Lists are joined once when data loads, so filtering, sorting and exports all see text.
export function listColumn(title, field) {
    return Object.assign(textColumn(title, field), {mutator: joinList});
}

export function positiveNumber(value) {
    return Number.isFinite(value) && value > 0 ? value : null;
}

function numberColumn(title, field, format) {
    return {title, field, visible: false, hozAlign: "right", sorter: "number",
        formatter: cell => {
            const value = positiveNumber(cell.getValue());
            return value === null ? "" : format(value);
        },
        accessorDownload: positiveNumber};
}

export function countColumn(title, field) {
    return numberColumn(title, field, String);
}

export function ageRatingColumn(field) {
    return numberColumn("Age rating", field, value => value + "+");
}

export function fileSizeColumn(title, field) {
    return numberColumn(title, field, formatFileSize);
}

// The catalog uses "0" for an unknown release date.
export function knownDate(value) {
    return typeof value === "string" && value !== "" && value !== "0" ? value : null;
}

export function releaseDateColumn(field, visible) {
    return {title: "Release date", field, visible, sorter: "date",
        sorterParams: {format: "YYYY-MM-DD", alignEmptyValues: "bottom"},
        formatter: cell => escapeHtml(knownDate(cell.getValue()) || ""),
        accessorDownload: knownDate};
}

export function catalogColumns(prefix, fields) {
    return [
        textColumn("Publisher", prefix + fields.publisher),
        listColumn("Genres", prefix + fields.genres),
        listColumn("Languages", prefix + fields.languages),
        countColumn("Players", prefix + fields.players),
        ageRatingColumn(prefix + fields.rating),
        listColumn("Rating descriptors", prefix + fields.ratingContent)
    ];
}
