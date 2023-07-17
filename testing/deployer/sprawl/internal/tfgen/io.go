package tfgen

import (
	"bytes"
	"os"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rboyer/safeio"
)

func WriteHCLResourceFile(
	logger hclog.Logger,
	res []Resource,
	path string,
	perm os.FileMode,
) (UpdateResult, error) {
	var text []string
	for _, r := range res {
		val, err := r.Render()
		if err != nil {
			return UpdateResultNone, err
		}
		text = append(text, strings.TrimSpace(val))
	}

	body := strings.Join(text, "\n\n")

	// Ensure it looks tidy
	out := hclwrite.Format(bytes.TrimSpace([]byte(body)))

	return UpdateFileIfDifferent(logger, out, path, perm)
}

type UpdateResult int

const (
	UpdateResultNone UpdateResult = iota
	UpdateResultCreated
	UpdateResultModified
)

func UpdateFileIfDifferent(
	logger hclog.Logger,
	body []byte,
	path string,
	perm os.FileMode,
) (UpdateResult, error) {
	prev, err := os.ReadFile(path)

	result := UpdateResultNone
	if err != nil {
		if !os.IsNotExist(err) {
			return result, err
		}
		logger.Info("writing new file", "path", path)
		result = UpdateResultCreated
	} else {
		// loaded
		if bytes.Equal(body, prev) {
			return result, nil
		}
		logger.Info("file has changed", "path", path)
		result = UpdateResultModified
	}

	_, err = safeio.WriteToFile(bytes.NewReader(body), path, perm)
	return result, err
}
