// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalogv2

import (
	"flag"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

var (
	dev1_17Images = topology.Images{
		ConsulCE:         "hashicorppreview/consul:1.17-dev",
		ConsulEnterprise: "hashicorppreview/consul-enterprise:1.17-dev",
		Dataplane:        "hashicorppreview/consul-dataplane:1.3-dev",
	}
)

var flagUseDevImages = flag.Bool("dev-images", false, "set to enable 1.17 dev images for everything")

// TODO(rb): bump the testing/deployer default versions when the various components have
// released versions and then this shim can be deleted.
func getImages() topology.Images {
	if *flagUseDevImages {
		return dev1_17Images
	}
	images := utils.TargetImages()
	images.Dataplane = dev1_17Images.Dataplane
	return images
}
