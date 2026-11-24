import assert from "node:assert/strict";
import test from "node:test";
import {
    ROW_NUMBER_FIELD,
    applyTableLayout,
    captureTableLayout,
    columnLabel,
    columnMenuItems,
    escapeHtml,
    formatFileSize,
    joinList,
    menuPosition,
    savedWidthFields,
} from "../frontend/table_layout.js";
import {
    ageRatingColumn,
    catalogColumns,
    countColumn,
    fileSizeColumn,
    knownDate,
    listColumn,
    releaseDateColumn,
    rowNumberColumn,
    ROW_METADATA_FIELDS,
} from "../frontend/table_columns.js";

const definitions = () => [
    {field: ROW_NUMBER_FIELD, formatter: "rownum"},
    {title: "Title", field: "name", widthGrow: 3},
    {title: "Title ID", field: "titleId"},
    {title: "Version", field: "version", visible: true},
    {title: "Publisher", field: "publisher", visible: false},
];
const fields = columns => columns.map(column => column.field);

function component(field, {visible = true, width = 100, title} = {}) {
    return {
        getField: () => field,
        isVisible: () => visible,
        getWidth: () => width,
        getDefinition: () => ({title}),
        show() { visible = true; },
        hide() { visible = false; }
    };
}

test("no saved layout keeps the default columns", () => {
    assert.deepEqual(applyTableLayout(definitions(), undefined), definitions());
    assert.deepEqual(applyTableLayout(definitions(), {columns: "bad"}), definitions());
});

test("saved order, visibility and widths are applied; new columns follow and stale ones are ignored", () => {
    const columns = applyTableLayout(definitions(), {columns: [
        {field: "publisher", visible: true, width: 180},
        {field: "removed", visible: true},
        {field: "name", visible: false, width: -5},
        {field: "name", visible: true},
        {field: ROW_NUMBER_FIELD, visible: true},
        null,
        {field: ""},
    ]});

    assert.deepEqual(fields(columns), ["publisher", "name", ROW_NUMBER_FIELD, "titleId", "version"]);
    assert.equal(columns[0].visible, true);
    assert.equal(columns[0].width, 180);
    assert.equal(columns[1].visible, false);
    assert.equal(columns[1].width, undefined, "invalid widths are not applied");
    assert.equal(columns[1].widthGrow, 3);
    assert.equal(columns[4].visible, true, "columns missing from the layout keep their default visibility");
});

test("columns hidden by an application setting stay hidden", () => {
    const columns = applyTableLayout(definitions(), {columns: [{field: "version", visible: true}]}, ["version"]);
    assert.equal(columns.find(column => column.field === "version").visible, false);
});

test("applying a layout does not modify the default definitions", () => {
    const original = definitions();
    applyTableLayout(original, {columns: [{field: "publisher", visible: true, width: 90}]});
    assert.deepEqual(original, definitions());
});

test("captured layouts save visibility for every column but widths only for resized ones", () => {
    const columns = [
        component(ROW_NUMBER_FIELD),
        component("name", {width: 240.6}),
        component("publisher", {visible: false}),
        component("version", {visible: false}),
        component(undefined),
        component("titleId", {width: 0}),
    ];
    const layout = captureTableLayout(columns, new Set(["name", "titleId"]), {columns: [{field: "version", visible: true}]}, ["version"]);

    assert.deepEqual(layout, {columns: [
        {field: ROW_NUMBER_FIELD, visible: true},
        {field: "name", visible: true, width: 241},
        {field: "publisher", visible: false},
        {field: "version", visible: true},
        {field: "titleId", visible: true},
    ]});
    const unsaved = captureTableLayout([component("version", {visible: false})], new Set(), undefined, ["version"]);
    assert.deepEqual(unsaved.columns, [{field: "version", visible: true}], "a locked column defaults to visible when nothing was saved");
});

test("saved widths count as user-resized so they are kept on the next save", () => {
    assert.deepEqual([...savedWidthFields({columns: [{field: "name", width: 120}, {field: "path", width: 0}, {field: "icon"}]})], ["name"]);
    assert.equal(savedWidthFields(undefined).size, 0);
});

test("the header menu toggles columns, protects the last visible one and resets", () => {
    const toggled = [];
    let resets = 0;
    const columns = [
        component(ROW_NUMBER_FIELD),
        component("name", {title: "<b>Title</b>"}),
        component("version"),
        component("publisher", {visible: false, title: "Publisher"}),
    ];
    const items = columnMenuItems(columns, {
        labels: {[ROW_NUMBER_FIELD]: "Row number"},
        hiddenFields: ["version"],
        onToggle: (column, visible) => toggled.push([column.getField(), visible]),
        onReset: () => resets++,
    });

    assert.equal(items.length, 5, "three toggles, a separator and reset");
    assert.match(items[0].label, /&#10003;<\/span>Row number$/);
    assert.match(items[1].label, /&lt;b&gt;Title&lt;\/b&gt;$/, "titles are escaped");
    assert.match(items[2].label, /"><\/span>Publisher$/);
    assert.equal(items[0].disabled, false);
    assert.deepEqual(items[3], {separator: true});

    items[1].action();
    items[2].action();
    items[4].action();
    assert.deepEqual(toggled, [["name", false], ["publisher", true]]);
    assert.equal(resets, 1);

    const single = columnMenuItems([component("name"), component("path", {visible: false})], {onToggle() {}, onReset() {}});
    assert.equal(single[0].disabled, true, "the only visible column cannot be hidden");
    assert.equal(single[1].disabled, false);
});

test("column labels fall back from configured labels to titles to fields", () => {
    assert.equal(columnLabel(component("icon"), {icon: "Thumbnail"}), "Thumbnail");
    assert.equal(columnLabel(component("name", {title: "Title"}), {}), "Title");
    assert.equal(columnLabel({getField: () => "path"}, undefined), "path");
});

test("list values are joined without blanks or repeats", () => {
    assert.equal(joinList(["zh", "en", " zh ", "", null, 4, "ja"]), "zh, en, ja");
    assert.equal(joinList("already joined"), "already joined");
    assert.equal(joinList(null), "");
    assert.equal(joinList(undefined), "");
    assert.equal(escapeHtml(`<a href="x">'&'</a>`), "&lt;a href=&quot;x&quot;&gt;&#39;&amp;&#39;&lt;/a&gt;");
});

test("file sizes are formatted for display", () => {
    assert.equal(formatFileSize(0), "");
    assert.equal(formatFileSize(-4), "");
    assert.equal(formatFileSize(undefined), "");
    assert.equal(formatFileSize(512), "512 B");
    assert.equal(formatFileSize(1536), "1.5 KB");
    assert.equal(formatFileSize(2192572416), "2.0 GB");
    assert.equal(formatFileSize(150 * 1024 * 1024), "150 MB");
    assert.equal(formatFileSize(5 * 1024 ** 5), "5120 TB");
});

test("optional catalog columns start hidden and format, sort and export their values", () => {
    const cell = value => ({getValue: () => value});
    for (const column of catalogColumns("", ROW_METADATA_FIELDS)) {
        assert.equal(column.visible, false, column.title);
    }
    assert.deepEqual(catalogColumns("Attributes.", ROW_METADATA_FIELDS).map(column => column.field)[0], "Attributes.publisher");

    assert.equal(listColumn("Genres", "genres").mutator(["Action", "Action", "RPG"]), "Action, RPG");

    const players = countColumn("Players", "players");
    assert.equal(players.formatter(cell(4)), "4");
    assert.equal(players.formatter(cell(0)), "");
    assert.equal(players.accessorDownload(0), null);
    assert.equal(players.accessorDownload(2), 2);

    const rating = ageRatingColumn("age_rating");
    assert.equal(rating.title, "Age rating");
    assert.equal(rating.formatter(cell(12)), "12+");
    assert.equal(rating.formatter(cell(null)), "");

    const size = fileSizeColumn("File size", "file_size");
    assert.equal(size.formatter(cell(2048)), "2.0 KB");
    assert.equal(size.accessorDownload(-1), null);

    const release = releaseDateColumn("release_date", true);
    assert.equal(release.visible, true);
    assert.equal(release.formatter(cell("2024-01-02")), "2024-01-02");
    assert.equal(release.formatter(cell("0")), "");
    assert.equal(release.formatter(cell("<i>")), "&lt;i&gt;");
    assert.equal(release.accessorDownload("0"), null);
    assert.equal(knownDate(20240102), null);

    assert.deepEqual(rowNumberColumn(), {formatter: "rownum", field: ROW_NUMBER_FIELD, headerSort: false, download: false});
});

test("column menus stay inside the window", () => {
    const viewport = {width: 1200, height: 600};
    assert.deepEqual(menuPosition({x: 300, y: 100}, {width: 200, height: 300}, viewport), {top: 100, left: 300},
        "a menu that fits opens at the pointer");
    assert.deepEqual(menuPosition({x: 1150, y: 160}, {width: 200, height: 584}, viewport), {top: 8, left: 992},
        "a tall menu near the bottom-right is shifted up and left, not flipped above the window");
    assert.deepEqual(menuPosition({x: 2, y: 590}, {width: 200, height: 100}, viewport), {top: 492, left: 8});
});
