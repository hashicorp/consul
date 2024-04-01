// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cli

import (
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
)

// Functions to set up colorized output.
const (
	noColor = -1
)

func colorize(message string, uc UiColor) string {
	if uc.Code == noColor {
		return message
	}

	attr := []color.Attribute{color.Attribute(uc.Code)}
	if uc.Bold {
		attr = append(attr, color.Bold)
	}

	return color.New(attr...).SprintFunc()(message)
}

// UiColor is a posix shell color code to use.
type UiColor struct {
	Code int
	Bold bool
}

// A list of colors that are useful. These are all non-bolded by default.
var (
	UiColorNone    UiColor = UiColor{noColor, false}
	UiColorRed             = UiColor{int(color.FgHiRed), false}
	UiColorGreen           = UiColor{int(color.FgHiGreen), false}
	UiColorYellow          = UiColor{int(color.FgHiYellow), false}
	UiColorBlue            = UiColor{int(color.FgHiBlue), false}
	UiColorMagenta         = UiColor{int(color.FgHiMagenta), false}
	UiColorCyan            = UiColor{int(color.FgHiCyan), false}
)

// Functions that are useful for table output.
const (
	Yellow = "yellow"
	Green  = "green"
	Red    = "red"
)

var colorMapping = map[string]int{
	Green:  tablewriter.FgGreenColor,
	Yellow: tablewriter.FgYellowColor,
	Red:    tablewriter.FgRedColor,
}

// Table is passed to UI.Table to provide a nicely formatted table.
type Table struct {
	Headers []string
	Rows    [][]Cell
}

// Cell is a single entry for a table.
type Cell struct {
	Value string
	Color string
}

// NewTable creates a new Table structure that can be used with UI.Table.
func NewTable(headers ...string) *Table {
	return &Table{
		Headers: headers,
	}
}

// AddRow adds a row to the table.
func (t *Table) AddRow(cols []string, colors []string) {
	var row []Cell

	for i, col := range cols {
		if i < len(colors) {
			row = append(row, Cell{Value: col, Color: colors[i]})
		} else {
			row = append(row, Cell{Value: col})
		}
	}

	t.Rows = append(t.Rows, row)
}
