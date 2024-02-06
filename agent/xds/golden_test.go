// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	gtest "github.com/hashicorp/consul/internal/testing/golden"
	"github.com/hashicorp/consul/proto/private/prototest"
)

// goldenSimple is just for read/write access to a golden file that is not
// envoy specific.
func goldenSimple(t *testing.T, name, got string) string {
	return golden(t, name, "", "", got)
}

// goldenEnvoy is a special variant of golden() that silos each named test by
// each supported envoy version
func goldenEnvoy(t *testing.T, name, envoyVersion, latestEnvoyVersion, got string) string {
	t.Helper()

	require.NotEmpty(t, envoyVersion)

	// We'll need both the name of this golden file for the requested version
	// of envoy AND the latest version of envoy due to how the golden file
	// coalescing works below when there is no xDS generated skew across envoy
	// versions.
	subname := goldenEnvoyVersionName(t, envoyVersion)

	latestSubname := "latest"
	if envoyVersion == latestEnvoyVersion {
		subname = "latest"
	}

	return golden(t, name, subname, latestSubname, got)
}

func goldenEnvoyVersionName(t *testing.T, envoyVersion string) string {
	t.Helper()

	require.NotEmpty(t, envoyVersion)

	// We do version sniffing on the complete version, but only generate
	// golden files ignoring the patch portion
	version := version.Must(version.NewVersion(envoyVersion))
	segments := version.Segments()
	require.Len(t, segments, 3)

	return fmt.Sprintf("envoy-%d-%d-x", segments[0], segments[1])
}

// golden reads and optionally writes the expected data to the golden file,
// returning the contents as a string.
//
// The golden file is named with two components the "name" and the "subname".
// In the common case of xDS tests the "name" component is the logical name of
// the test itself, and the "subname" is derived from the envoy major version.
//
// If latestSubname is specified we use that as a fallback source of comparison
// if the specific golden file referred to by subname is absent.
//
// If the -update flag is passed when executing the tests then the contents of
// the "got" argument are written to the golden file on disk. If the
// latestSubname argument is specified in this mode and the generated content
// matches that of the latest generated content then the specific golden file
// referred to by 'subname' is deleted to avoid unnecessary duplication in the
// testdata directory.
func golden(t *testing.T, name, subname, latestSubname, got string) string {
	t.Helper()

	var (
		currentFile string
		latestFile  string
		isLatest    bool
	)

	if subname != "" {
		currentFile = gtest.Filepath(strings.Join([]string{name, subname}, "."))
	} else {
		currentFile = gtest.Filepath(name)
	}

	isLatest = subname == latestSubname

	if latestSubname == "" {
		latestFile = gtest.Filepath(name)
	} else {
		latestFile = gtest.Filepath(strings.Join([]string{name, latestSubname}, "."))
	}

	latestExpected := gtest.GetAtFilePath(t, latestFile)

	// Handle easy updates to the golden files in the agent/xds/testdata
	// directory.
	//
	// To trim down PRs, we only create per-version golden files if they differ
	// from the latest version.

	if gtest.ShouldUpdate() && got != "" {
		var gotInterface, latestExpectedInterface interface{}
		json.Unmarshal([]byte(got), &gotInterface)
		json.Unmarshal([]byte(latestExpected), &latestExpectedInterface)

		// Remove non-latest golden files if they are the same as the latest one.
		if !isLatest && assert.ObjectsAreEqualValues(gotInterface, latestExpectedInterface) {
			// In update mode we erase a golden file if it is identical to
			// the golden file corresponding to the latest version of
			// envoy.
			err := os.Remove(currentFile)
			if err != nil && !os.IsNotExist(err) {
				require.NoError(t, err)
			}
			return got
		}

		gtest.WriteContentsToFilePath(t, got, currentFile)

		return got
	}

	expected, err := os.ReadFile(currentFile)
	if latestExpected != "" && os.IsNotExist(err) {
		// In readonly mode if a specific golden file isn't found, we fallback
		// on the latest one.
		return latestExpected
	}
	require.NoError(t, err)
	return string(expected)
}

func protoToJSON(t *testing.T, pb proto.Message) string {
	return prototest.ProtoToJSON(t, pb)
}
