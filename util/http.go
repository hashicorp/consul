package util

import (
	"errors"
	"net/http"
	"strings"
)

const squid503 = `Unexpected response code: 503`

func TruncateSquidError(err error) error {
	if IsSquid503(err) {
		return errors.New("Squid: " + squid503)
	}
	return err
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
