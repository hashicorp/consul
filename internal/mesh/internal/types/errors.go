package types

import (
	"errors"
)

var (
	errInvalidPort               = errors.New("port number is outside the range 1 to 65535")
	errInvalidExposePathProtocol = errors.New("invalid protocol: only HTTP and HTTP2 protocols are allowed")
	errMissingProxyConfigData    = errors.New("at least one of \"bootstrap_config\" or \"dynamic_config\" fields must be set")
)
