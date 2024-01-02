// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resourcetest

import (
	"fmt"
	"io/fs"

	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

// ParseResourcesFromFilesystem will walk the filesystem at the given path
// and parse all files as protobuf/JSON resources.
func ParseResourcesFromFilesystem(t T, files fs.FS, path string) []*pbresource.Resource {
	t.Helper()

	var resources []*pbresource.Resource
	err := fs.WalkDir(files, path, func(fpath string, dent fs.DirEntry, _ error) error {
		if dent.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(files, fpath)
		if err != nil {
			return err
		}

		var res pbresource.Resource
		err = protojson.Unmarshal(data, &res)
		if err != nil {
			return fmt.Errorf("error decoding data from %s: %w", fpath, err)
		}

		resources = append(resources, &res)
		return nil
	})

	require.NoError(t, err)
	return resources
}
