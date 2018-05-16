package hrf

import (
	"github.com/v2pro/plz/countlog/spi"
	"github.com/v2pro/plz/countlog/output"
	"github.com/v2pro/plz/msgfmt"
	"strings"
)

// Format is human readable format
type Format struct {
	ShowTimestamp bool
}

func (format *Format) FormatterOf(site *spi.LogSite) output.Formatter {
	var formatters output.Formatters
	formatters = append(formatters, &levelFormatter{})
	if strings.HasPrefix(site.Event, "event!") {
		formatters = append(formatters, fixedFormatter(site.Event[len("event!"):]))
	} else if strings.HasPrefix(site.Event, "callee!") {
		formatters = append(formatters, fixedFormatter(site.Event[len("callee!"):]))
	} else {
		formatters = append(formatters,
			&propFormatter{msgfmt.FormatterOf(site.Event, site.Sample)})
	}
	if format.ShowTimestamp {
		formatters = append(formatters, fixedFormatter("\n\x1b[90;1mtimestamp: "))
		formatters = append(formatters, &timestampFormatter{})
		formatters = append(formatters, fixedFormatter("\x1b[0m"))
	}
	formatters = append(formatters, &errorFormatter{})
	ctx := site.LogContext()
	if ctx != nil {
		for i := 0; i < len(ctx.Properties); i += 2 {
			key := ctx.Properties[i].(string)
			formatters = append(formatters, fixedFormatter("\x1b[90;1m"))
			formatters = append(formatters, &ctxFormatter{
				msgfmt.FormatterOf("\n"+key+": {"+key+"}", ctx.Properties),
			})
			formatters = append(formatters, fixedFormatter("\x1b[0m"))
		}
	}
	for i := 0; i < len(site.Sample); i += 2 {
		key := site.Sample[i].(string)
		formatters = append(formatters, fixedFormatter("\x1b[90;1m"))
		if key == "timer" {
			formatters = append(formatters, &timerFormatter{idx: i+1})
		} else {
			formatters = append(formatters, &propFormatter{
				msgfmt.FormatterOf("\n"+key+": {"+key+"}", site.Sample),
			})
		}
		formatters = append(formatters, fixedFormatter("\x1b[0m"))
	}
	formatters = append(formatters, fixedFormatter("\x1b[90;1m"))
	formatters = append(formatters, locationFormatter("\nlocation: "+site.Location()))
	formatters = append(formatters, fixedFormatter("\x1b[0m"))
	formatters = append(formatters, fixedFormatter("\n"))
	return formatters
}
