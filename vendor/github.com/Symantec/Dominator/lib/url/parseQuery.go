package url

import (
	"net/url"
	"strings"
)

func parseQuery(URL *url.URL) ParsedQuery {
	var parsedQuery ParsedQuery
	parsedQuery.Flags = make(map[string]struct{})
	parsedQuery.Table = make(map[string]string)
	for _, pair := range strings.Split(URL.RawQuery, "&") {
		splitPair := strings.Split(pair, "=")
		if len(splitPair) == 1 {
			parsedQuery.Flags[splitPair[0]] = struct{}{}
		}
		if len(splitPair) == 2 {
			parsedQuery.Table[splitPair[0]] = splitPair[1]
		}
	}
	return parsedQuery
}
