// Stand-ins for the app.js helpers that loadTab uses, for tests that run loadTab in a
// vm sandbox. Text is "measured" at 8px per character, and laid out (readableColumn) as if
// it could break anywhere.
import * as tableColumns from "../frontend/table_columns.js";
import {contentMinWidth, digitsWidth, linesWidth, longestWordWidth} from "../frontend/table_layout.js";

export const measureText = text => String(text).length * 8;

const fieldValues = (definition, rows) => {
    const path = definition.field.split(".");
    return rows.map(row => path.reduce((value, key) => value == null ? value : value[key], row));
};
const withMinWidth = (definition, minWidth) =>
    minWidth > (definition.minWidth || 0) ? Object.assign({}, definition, {minWidth}) : definition;

export const tableHelpers = {
    ...tableColumns,
    tableLayoutOptions: options => options,
    rowNumber: () => tableColumns.rowNumberColumn(digitsWidth(3, measureText)),
    oneLineColumn: (definition, rows) => withMinWidth(definition, contentMinWidth(fieldValues(definition, rows), measureText)),
    readableColumn(definition, rows, text) {
        const values = text ? rows.map(text) : fieldValues(definition, rows);
        // Lines as if text broke anywhere: its width spread over the column's.
        const countLines = (width, texts) => texts.map(value => Math.ceil(measureText(value) / width));
        return withMinWidth(definition, Math.max(longestWordWidth(values, measureText),
            linesWidth(values, tableColumns.MAX_ROW_LINES, measureText, countLines)));
    }
};
