// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topoutil

import (
	"os"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

func TargetImages() topology.Images {
	// Start with no preferences.
	var images topology.Images
	if !runningInCI() {
		// Until 1.17 GAs, we want the pre-release versions for these tests,
		// run outside of CI for convenience.
		images = topology.Images{
			ConsulCE:         HashicorpDockerProxy + "/hashicorppreview/consul:1.17-dev",
			ConsulEnterprise: HashicorpDockerProxy + "/hashicorppreview/consul-enterprise:1.17-dev",
			Dataplane:        HashicorpDockerProxy + "/hashicorppreview/consul-dataplane:1.3-dev",
		}
	}

	// We want the image overridden by the local build produced by
	// 'make test-compat-integ-setup' or 'make dev-docker'.
	testImages := utils.TargetImages()
	images = images.OverrideWith(testImages)

	return images
}

func runningInCI() bool {
	return os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("CI") != ""
}
