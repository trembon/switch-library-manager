import assert from "node:assert/strict";
import test from "node:test";
import {
    ROW_NUMBER_FIELD,
    applyTableLayout,
    captureTableLayout,
    columnLabel,
    columnMenuItems,
    contentMinWidth,
    digitsWidth,
    escapeHtml,
    formatFileSize,
    longestWordWidth,
    joinList,
    linesWidth,
    headerMinWidth,
    menuPosition,
    savedWidthFields,
    serialSaver,
    trimOverflow,
    withHeaderMinWidths,
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
    THUMBNAIL_WIDTH,
    thumbnailColumn,
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

test("a layout that would hide every column shows the defaults instead", () => {
    // Only Version left visible, then "hide version string" turned on: nothing would be
    // left to right-click.
    const layout = {columns: definitions().map(column => ({field: column.field, visible: column.field === "version"}))};
    const columns = applyTableLayout(definitions(), layout, ["version"]);
    assert.deepEqual(columns.filter(column => column.visible !== false).map(column => column.field),
        [ROW_NUMBER_FIELD, "name", "titleId"], "the default columns, without the locked one");

    // Without a lock the saved choice stands.
    const unlocked = applyTableLayout(definitions(), layout);
    assert.deepEqual(unlocked.filter(column => column.visible !== false).map(column => column.field), ["version"]);

    // If every column is hidden by default too, the first unlocked one is shown.
    const hiddenByDefault = [{field: "a", visible: false}, {field: "b", visible: false}];
    assert.deepEqual(applyTableLayout(hiddenByDefault, undefined, ["a"]).map(column => column.visible), [false, true]);
});

test("a column exported while hidden by a setting is left out of exports once the user hides it", () => {
    const columns = () => [{title: "Title", field: "name"}, {title: "Version", field: "version", download: true}];
    const version = (layout, locked = ["version"]) => applyTableLayout(columns(), layout, locked).find(column => column.field === "version");

    // Hidden by "hide version string" only: still exported, as before.
    assert.equal(version(undefined).download, true);
    assert.equal(version({columns: [{field: "version", visible: true}]}).download, true);
    // Hidden by the user before the setting was turned on: left out.
    assert.equal(version({columns: [{field: "version", visible: false}]}).download, false);
    assert.equal(version({columns: [{field: "version", visible: false}]}).visible, false);

    // Not locked: exported exactly when visible, so hiding it from the menu leaves it out.
    assert.equal("download" in version(undefined, []), false);
    assert.equal("download" in version({columns: [{field: "version", visible: false}]}, []), false);
    // Columns that are never exported stay that way.
    const icon = applyTableLayout([{field: "icon", download: false}], {columns: [{field: "icon", visible: true}]})[0];
    assert.equal(icon.download, false);

    // A layout with every column hidden falls back to the defaults; the column shown again
    // is exported, because nothing marks it as left out.
    const fallback = applyTableLayout(columns(), {columns: [{field: "name", visible: false}, {field: "version", visible: false}]});
    assert.deepEqual(fallback.map(column => [column.field, column.visible, column.download]),
        [["name", true, undefined], ["version", true, undefined]]);
    // No option Tabulator doesn't know is added (it warns about unknown column options).
    for (const column of fallback.concat(version(undefined), version({columns: [{field: "version", visible: false}]}))) {
        assert.deepEqual(Object.keys(column).filter(name => !["title", "field", "download", "visible", "width"].includes(name)), []);
    }
});

test("layout saves are sent at once, one at a time per table, newest last", async () => {
    const sent = [];
    const pending = [];
    const save = serialSaver((key, layout) => {
        sent.push([key, layout]);
        return new Promise((resolve, reject) => pending.push({key, resolve, reject}));
    });
    const settle = () => new Promise(resolve => setTimeout(resolve, 0));
    const inFlight = key => pending.splice(pending.findIndex(entry => entry.key === key), 1)[0];

    save("library", "A");
    assert.deepEqual(sent, [["library", "A"]], "the first change is sent straight away, not after a delay");
    save("library", "B");
    save("library", "C");
    save("issues", "X");
    assert.deepEqual(sent, [["library", "A"], ["issues", "X"]], "other tables are not held up; this one waits");

    inFlight("library").resolve();
    await settle();
    assert.deepEqual(sent.at(-1), ["library", "C"], "only the newest waiting layout is sent");
    assert.equal(sent.filter(([key]) => key === "library").length, 2);

    // A reset sent while a save is in progress lands after it.
    save("library", {columns: []});
    assert.deepEqual(sent.at(-1), ["library", "C"], "the reset waits for the save in progress");
    inFlight("library").reject(new Error("disk full"));
    await settle();
    assert.deepEqual(sent.at(-1), ["library", {columns: []}], "a failed save does not stop the next one");

    // Nothing waiting: the next change is sent at once again.
    for (const entry of pending.splice(0)) {
        entry.resolve();
    }
    await settle();
    save("library", "D");
    assert.deepEqual(sent.at(-1), ["library", "D"]);

    // A send that throws does not block later saves either.
    const thrown = [];
    const failing = serialSaver((key, layout) => {
        thrown.push(layout);
        if (layout === 1) {
            throw new Error("boom");
        }
    });
    const logged = [];
    const consoleError = console.error;
    console.error = (...values) => logged.push(values);
    try {
        failing("t", 1);
        failing("t", 2);
        await settle();
    } finally {
        console.error = consoleError;
    }
    assert.deepEqual(thrown, [1, 2]);
    // The error is logged, not dropped.
    assert.equal(logged.length, 1);
    assert.equal(logged[0][1], "t");
    assert.equal(logged[0][2].message, "boom");
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

    assert.deepEqual(rowNumberColumn(), {formatter: "rownum", field: ROW_NUMBER_FIELD, headerSort: false, download: false, hozAlign: "center"});
    assert.equal(rowNumberColumn(44).width, 44);
    const thumbnail = thumbnailColumn("icon");
    assert.deepEqual(Object.assign({}, thumbnail, {formatter: undefined}), {field: "icon", visible: true, download: false,
        formatter: undefined, headerSort: false, width: THUMBNAIL_WIDTH, hozAlign: "center"});
    assert.equal(thumbnailColumn("game.iconUrl", false).visible, false);
});

test("thumbnails show the art, or an empty cell instead of a broken image", () => {
    const previous = globalThis.document;
    const listeners = [];
    globalThis.document = {createElement: tag => ({tag, style: {}, attributes: {},
        setAttribute(name, value) { this.attributes[name] = value; },
        addEventListener(event, listener) { listeners.push([event, listener]); }})};
    try {
        const {formatter} = thumbnailColumn("icon");
        let normalized = 0;
        const cell = value => ({getValue: () => value, getRow: () => ({normalizeHeight: () => normalized++})});
        for (const missing of [undefined, null, "", "  ", 42]) {
            assert.equal(formatter(cell(missing)), "", `no image for ${JSON.stringify(missing)}`);
        }
        const image = formatter(cell("https://example.invalid/art.jpg"));
        assert.equal(image.tag, "img");
        assert.equal(image.attributes.src, "https://example.invalid/art.jpg");
        assert.deepEqual(image.style, {height: "60px", width: "60px"});
        assert.equal(listeners.length, 1);
        assert.equal(listeners[0][0], "load");
        listeners[0][1]();
        assert.equal(normalized, 1, "the row is resized once the art loads");
    } finally {
        globalThis.document = previous;
    }
});

test("ID and row-number columns are sized to their content", () => {
    const measured = [];
    const measure = text => {
        measured.push(text);
        return text.length * 8 + (text.includes("D") ? 0.4 : 0);
    };
    assert.equal(contentMinWidth(["0100000000001000", "0100D00000001000", "0100D00000001000", "", null, 7], measure),
        Math.ceil(16 * 8 + 0.4) + 20, "the widest ID plus cell padding");
    assert.equal(measured.length, 2, "repeated and non-text values are not measured");
    assert.equal(contentMinWidth([], measure), 0);
    assert.equal(contentMinWidth([undefined, ""], measure), 0);

    const digit = character => ({"1": 5, "8": 9})[character] ?? 7;
    assert.equal(digitsWidth(3, digit), 3 * 9 + 20, "three of the widest digit");
});

test("text and list columns fit their longest word", () => {
    const measure = text => text.length * 8;
    assert.equal(longestWordWidth(["Nintendo", "Bandai Namco Entertainment", null], measure), 13 * 8 + 20,
        "the longest word of any value plus cell padding");
    assert.equal(longestWordWidth([["Multiplayer", "Action"], ["Simulation"]], measure), "Multiplayer,".length * 8 + 20,
        "lists are measured as displayed, joined with commas");
    assert.equal(longestWordWidth(["A_very_long_file_name_without_any_spaces.nsp"], measure), 150,
        "one long word cannot take over the table");
    assert.equal(longestWordWidth(["x".repeat(30)], measure, 120), 120);
    assert.equal(longestWordWidth([], measure), 0);
    assert.equal(longestWordWidth([7, "", [], undefined], measure), 0);
});

test("text columns are widened until every value fits in three lines", () => {
    const measure = text => text.length * 10;
    const laidOut = [];
    // Lines as if text broke anywhere: its width spread over the column's.
    const countLines = (width, texts) => {
        laidOut.push(...texts);
        return texts.map(text => Math.ceil(measure(text) / width));
    };
    // The 300px text needs 100px for three lines; the cell adds padding and slack.
    assert.equal(linesWidth(["x".repeat(30), "short", ["Action", "RPG"]], 3, measure, countLines), 100 + 20);
    assert.ok(!laidOut.includes("short"), "values that fit on one line are not laid out");
    assert.equal(linesWidth([["Board Game", "Strategy"]], 2, measure, countLines), Math.ceil(200 / 2) + 20,
        "lists are measured as displayed, joined with commas");
    // A layout that wraps more than the width suggests (words that do not split) widens further.
    const wordWrap = (width, texts) => texts.map(text => text.split(" ").length > 1 && width < 60 ? 4 : 1);
    assert.equal(linesWidth(["aaaaa bbbbb"], 3, measure, wordWrap), 60 + 20);
    assert.equal(linesWidth([7, "", null, []], 3, measure, countLines), 0);
});

test("column menus stay inside the window", () => {
    const viewport = {width: 1200, height: 600};
    assert.deepEqual(menuPosition({x: 300, y: 100}, {width: 200, height: 300}, viewport), {top: 100, left: 300},
        "a menu that fits opens at the pointer");
    assert.deepEqual(menuPosition({x: 1150, y: 160}, {width: 200, height: 584}, viewport), {top: 8, left: 992},
        "a tall menu near the bottom-right is shifted up and left, not flipped above the window");
    assert.deepEqual(menuPosition({x: 2, y: 590}, {width: 200, height: 100}, viewport), {top: 492, left: 8});
});

test("columns are at least as wide as their header title", () => {
    const measure = text => text.length * 10;
    assert.equal(headerMinWidth("Rating descriptors", true, measure), 180 + 16 + 4 + 16, "the whole title plus padding and the sort arrow");
    assert.equal(headerMinWidth(" Rating   descriptors ", true, measure), 180 + 16 + 4 + 16, "spaces collapse as displayed");
    assert.equal(headerMinWidth("Players", false, measure), 70 + 16 + 4);
    assert.equal(headerMinWidth("  ", true, measure), 0);
    assert.equal(headerMinWidth(undefined, true, measure), 0);

    const columns = withHeaderMinWidths([
        {field: ROW_NUMBER_FIELD},
        {title: "Genres", field: "genres"},
        {title: "Title ID", field: "titleId", headerSort: false},
        {title: "Title", field: "name", minWidth: 500},
        {title: "Version", field: "version", width: 40},
    ], measure, true);
    assert.deepEqual(columns.map(column => column.minWidth), [undefined, 60 + 36, 80 + 20, 500, 70 + 36],
        "a fixed or saved width is raised to fit the title too");

    const unsortedTable = withHeaderMinWidths([{title: "Genres", field: "genres"}], measure, false);
    assert.equal(unsortedTable[0].minWidth, 60 + 20, "a table without sorting reserves no arrow space");
});

test("layout overflow is trimmed from flexible columns without going below their minimum", () => {
    const columns = [
        {width: 80, minWidth: 24, flexible: true},
        {width: 240, minWidth: 75, flexible: true},
        {width: 150, minWidth: 0, flexible: false},
        {width: 94, minWidth: 90, flexible: true},
    ];
    assert.deepEqual(trimOverflow(columns, 562), [80, 240, 150, 92], "the last flexible column absorbs a small overflow");
    assert.deepEqual(trimOverflow(columns, 557), [80, 237, 150, 90], "then the one before it, once the last is at its minimum");
    assert.deepEqual(trimOverflow(columns, 500), [80, 180, 150, 90], "earlier columns give the rest, down to their minimum");
    assert.deepEqual(trimOverflow(columns, 600), [80, 240, 150, 94], "a table that fits is unchanged");
    assert.deepEqual(trimOverflow(columns, 100), [24, 75, 150, 90], "columns never shrink below their minimum");
});
