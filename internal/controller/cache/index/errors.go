package index

import (
	"fmt"
)

type MissingRequiredIndexError struct {
	Name string
}

func (err MissingRequiredIndexError) Error() string {
	return fmt.Sprintf("the indexer produced no value for the required %q index", err.Name)
}
