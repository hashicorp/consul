package compact

import (
	"strings"
	"github.com/v2pro/plz/countlog/spi"
	"github.com/v2pro/plz/countlog/output"
	"github.com/v2pro/plz/msgfmt"
	"fmt"
)

type Format struct {
}

func (format *Format) FormatterOf(site *spi.LogSite) output.Formatter {
	eventName := site.Event
	sample := site.Sample
	var formatters output.Formatters
	if strings.HasPrefix(eventName, "event!") {
		formatters = append(formatters, fixedFormatter(eventName[len("event!"):]))
	} else if strings.HasPrefix(eventName, "callee!") {
		tag := "call " + eventName[len("callee!"):]
		formatters = append(formatters, fixedFormatter(tag))
	} else {
		formatters = append(formatters,
			&defaultFormatter{msgfmt.FormatterOf(eventName, site.Sample)})
	}
	formatters = append(formatters, &timestampFormatter{})
	for i := 0; i < len(sample); i += 2 {
		key := sample[i].(string)
		pattern := "||" + key + "={" + key + "}"
		formatters = append(formatters, &defaultFormatter{
			msgfmt.FormatterOf(pattern, sample),
		})
	}
	formatters = append(formatters, fixedFormatter(fmt.Sprintf(
		"||location=%s\n", site.Location())))
	return formatters
}
