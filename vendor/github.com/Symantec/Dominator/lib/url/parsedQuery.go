package url

func (pq ParsedQuery) outputType() uint {
	if value, ok := pq.Table["output"]; ok {
		switch value {
		case "json":
			return OutputTypeJson
		case "text":
			return OutputTypeText
		}
	}
	return OutputTypeHtml
}
