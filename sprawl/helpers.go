package sprawl

import (
	"github.com/hashicorp/consul-topology/util"
)

// Deprecated: see util
func TruncateSquidError(err error) error {
	return util.TruncateSquidError(err)
}

// Deprecated: see util
func IsSquid503(err error) bool {
	return util.IsSquid503(err)
}
