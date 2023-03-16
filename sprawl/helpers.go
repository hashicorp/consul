package sprawl

import (
	"errors"
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
