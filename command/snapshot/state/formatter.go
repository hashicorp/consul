package state

const (
	PrettyFormat string = "pretty"
	JSONFormat   string = "json"
)

func GetSupportedFormats() []string {
	return []string{PrettyFormat, JSONFormat}
}
