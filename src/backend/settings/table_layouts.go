package settings

import (
	"fmt"
	"strings"
)

// Keys of the GUI tables whose column layout can be saved.
const (
	TableLayoutLibrary        = "library"
	TableLayoutMissingGames   = "missing_games"
	TableLayoutMissingUpdates = "missing_updates"
	TableLayoutMissingDLC     = "missing_dlc"
	TableLayoutIssues         = "issues"
)

const (
	maxTableLayoutColumns     = 64
	maxTableLayoutFieldLength = 128
	maxTableLayoutColumnWidth = 4000
)

// ColumnLayout is one column of a saved table layout. Columns are stored in
// display order. Visible is always written so a column that is hidden by
// default stays visible once the user shows it. Width is only set for columns
// the user resized; zero lets the table size the column.
type ColumnLayout struct {
	Field   string `json:"field"`
	Visible bool   `json:"visible"`
	Width   int    `json:"width,omitempty"`
}

// TableLayout is the saved column order, visibility and widths of one table.
type TableLayout struct {
	Columns []ColumnLayout `json:"columns"`
}

// IsTableLayoutKey reports whether key names a table whose layout can be saved.
func IsTableLayoutKey(key string) bool {
	switch key {
	case TableLayoutLibrary, TableLayoutMissingGames, TableLayoutMissingUpdates, TableLayoutMissingDLC, TableLayoutIssues:
		return true
	default:
		return false
	}
}

// normalizeTableLayouts drops layouts for unknown tables and empty layouts,
// and sanitizes the rest. The settings file is user-editable, so every value
// is treated as untrusted. It returns nil when no layout remains.
func normalizeTableLayouts(layouts map[string]TableLayout) map[string]TableLayout {
	var result map[string]TableLayout
	for key, layout := range layouts {
		if !IsTableLayoutKey(key) {
			continue
		}
		layout = normalizeTableLayout(layout)
		if len(layout.Columns) == 0 {
			continue
		}
		if result == nil {
			result = make(map[string]TableLayout)
		}
		result[key] = layout
	}
	return result
}

// normalizeTableLayout keeps the first entry for each non-empty field, caps
// the number of columns and field length, and clamps widths to a sane range.
func normalizeTableLayout(layout TableLayout) TableLayout {
	columns := make([]ColumnLayout, 0, min(len(layout.Columns), maxTableLayoutColumns))
	seen := make(map[string]struct{}, len(layout.Columns))
	for _, column := range layout.Columns {
		if len(columns) == maxTableLayoutColumns {
			break
		}
		field := strings.TrimSpace(column.Field)
		if field == "" || len(field) > maxTableLayoutFieldLength {
			continue
		}
		if _, duplicate := seen[field]; duplicate {
			continue
		}
		seen[field] = struct{}{}
		column.Field = field
		column.Width = max(0, min(column.Width, maxTableLayoutColumnWidth))
		columns = append(columns, column)
	}
	return TableLayout{Columns: columns}
}

// WithTableLayout returns a copy of current in which the layout of table is
// replaced, or removed when layout has no columns (reset to defaults).
// current is not modified, so a failed save leaves the loaded settings as
// they were.
func WithTableLayout(current *AppSettings, table string, layout TableLayout) (*AppSettings, error) {
	if current == nil {
		return nil, fmt.Errorf("save %s table layout: settings are not loaded", table)
	}
	if !IsTableLayoutKey(table) {
		return nil, fmt.Errorf("save table layout: unknown table %q", table)
	}

	updated := *current
	layouts := make(map[string]TableLayout, len(current.GUI.TableLayouts)+1)
	for key, value := range current.GUI.TableLayouts {
		layouts[key] = value
	}
	layout = normalizeTableLayout(layout)
	if len(layout.Columns) == 0 {
		delete(layouts, table)
	} else {
		layouts[table] = layout
	}
	updated.GUI.TableLayouts = normalizeTableLayouts(layouts)
	return &updated, nil
}
