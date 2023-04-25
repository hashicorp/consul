package util

import (
	"bufio"
	"errors"
	"net/http"
	"strings"
)

const squid503 = `Unexpected response code: 503`
const squidErrorPage = `Stylesheet for Squid Error pages`

func TruncateSquidError(err error) error {
	return tidySquidError(err)
}

func tidySquidError(err error) error {
	if err == nil {
		return nil
	}
	if IsSquid503(err) {
		return errors.New("Squid: " + squid503)
	}
	msg := err.Error()
	if !strings.Contains(msg, squidErrorPage) {
		return err
	}

	var (
		kept []string
		scan = bufio.NewScanner(strings.NewReader(msg))

		inBody bool
	)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())

		if inBody {
			if strings.Contains(line, `<div id="footer">`) {
				break
			}
			kept = append(kept, line)
		} else {
			if strings.Contains(line, `<body`) {
				inBody = true
			}
		}
	}
	if scan.Err() != nil {
		return err // ignore attempt to tidy it
	}

	return errors.New(strings.Join(kept, "\n"))
}

func IsSquid503(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), squid503)
}

func SquidErrorHidingRoundTripper(t *http.Transport) http.RoundTripper {
	return &maskingRoundTripper{t}
}

type maskingRoundTripper struct {
	*http.Transport
}

func (t *maskingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		err = TruncateSquidError(err)
	}
	return resp, err
}
