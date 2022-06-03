package pbpeering

const (
	TypeURLService = "type.googleapis.com/consul.api.Service"
	TypeURLRoots   = "type.googleapis.com/consul.api.CARoots"
)

func KnownTypeURL(s string) bool {
	return s == TypeURLService || s == TypeURLRoots
}
