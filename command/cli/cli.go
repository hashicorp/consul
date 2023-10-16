// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cli

import (
	"fmt"
	"io"

	"github.com/olekukonko/tablewriter"

	mcli "github.com/mitchellh/cli"
)

// Ui implements the mitchellh/cli.Ui interface, while exposing the underlying
// io.Writer used for stdout and stderr.
// TODO: rename to UI to match golang style guide
type Ui interface {
	mcli.Ui
	Stdout() io.Writer
	Stderr() io.Writer
	ErrorOutput(string)
	HeaderOutput(string)
	WarnOutput(string)
	SuccessOutput(string)
	UnchangedOutput(string)
	Table(tbl *Table)
}

// BasicUI augments mitchellh/cli.BasicUi by exposing the underlying io.Writer.
type BasicUI struct {
	mcli.BasicUi
}

func (b *BasicUI) Stdout() io.Writer {
	return b.BasicUi.Writer
}

func (b *BasicUI) Stderr() io.Writer {
	return b.BasicUi.ErrorWriter
}

func (b *BasicUI) HeaderOutput(s string) {
	b.Output(colorize(fmt.Sprintf("==> %s", s), UiColorNone))
}

func (b *BasicUI) ErrorOutput(s string) {
	b.Output(colorize(fmt.Sprintf(" ! %s", s), UiColorRed))
}

func (b *BasicUI) WarnOutput(s string) {
	b.Output(colorize(fmt.Sprintf(" * %s", s), UiColorYellow))
}

func (b *BasicUI) SuccessOutput(s string) {
	b.Output(colorize(fmt.Sprintf(" ✓ %s", s), UiColorGreen))
}

func (b *BasicUI) UnchangedOutput(s string) {
	b.Output(colorize(fmt.Sprintf("  %s", s), UiColorNone))
}

// Command is an alias to reduce the diff. It can be removed at any time.
type Command mcli.Command

// Table implements UI.
func (u *BasicUI) Table(tbl *Table) {
	// Build our config and set our options

	table := tablewriter.NewWriter(u.Stdout())

	table.SetHeader(tbl.Headers)
	table.SetBorder(false)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("\t") // pad with tabs
	table.SetNoWhiteSpace(true)

	for _, row := range tbl.Rows {
		colors := make([]tablewriter.Colors, len(row))
		entries := make([]string, len(row))

		for i, ent := range row {
			entries[i] = ent.Value

			color, ok := colorMapping[ent.Color]
			if ok {
				colors[i] = tablewriter.Colors{color}
			}
		}

		table.Rich(entries, colors)
	}

	table.Render()
}
