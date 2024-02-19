// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topology

import (
	"strings"

	goversion "github.com/hashicorp/go-version"
)

var (
	MinVersionAgentTokenPartition = goversion.Must(goversion.NewVersion("v1.11.0"))
	MinVersionPeering             = goversion.Must(goversion.NewVersion("v1.13.0"))
	MinVersionTLS                 = goversion.Must(goversion.NewVersion("v1.12.0"))
)

type Images struct {
	// Consul is the image used for creating the container,
	// Use ChooseConsul() to control which image (ConsulCE or ConsulEnterprise) assign to Consul
	Consul string `json:",omitempty"`
	// ConsulCE sets the CE image
	ConsulCE string `json:",omitempty"`
	// consulVersion is the version part of Consul image,
	// e.g., if Consul image is hashicorp/consul-enterprise:1.15.0-ent,
	// consulVersion is 1.15.0-ent
	consulVersion string
	// ConsulEnterprise sets the ent image
	ConsulEnterprise string `json:",omitempty"`
	Envoy            string
	Dataplane        string
}

func (i Images) LocalDataplaneImage() string {
	if i.Dataplane == "" {
		return ""
	}

	img, tag, ok := strings.Cut(i.Dataplane, ":")
	if !ok {
		tag = "latest"
	}

	name := strings.ReplaceAll(img, "/", "-")

	// ex: local/hashicorp-consul-dataplane:1.1.0
	return "local/" + name + ":" + tag
}

func (i Images) LocalDataplaneTProxyImage() string {
	return spliceImageNamesAndTags(i.Dataplane, i.Consul, "tproxy")
}

func (i Images) EnvoyConsulImage() string {
	return spliceImageNamesAndTags(i.Consul, i.Envoy, "")
}

func spliceImageNamesAndTags(base1, base2, nameSuffix string) string {
	if base1 == "" || base2 == "" {
		return ""
	}

	img1, tag1, ok1 := strings.Cut(base1, ":")
	img2, tag2, ok2 := strings.Cut(base2, ":")
	if !ok1 {
		tag1 = "latest"
	}
	if !ok2 {
		tag2 = "latest"
	}

	name1 := strings.ReplaceAll(img1, "/", "-")
	name2 := strings.ReplaceAll(img2, "/", "-")

	if nameSuffix != "" {
		nameSuffix = "-" + nameSuffix
	}

	// ex: local/hashicorp-consul-and-envoyproxy-envoy:1.15.0-with-v1.26.2
	return "local/" + name1 + "-and-" + name2 + nameSuffix + ":" + tag1 + "-with-" + tag2
}

// TODO: what is this for and why do we need to do this and why is it named this?
func (i Images) ChooseNode(kind NodeKind) Images {
	switch kind {
	case NodeKindServer:
		i.Envoy = ""
		i.Dataplane = ""
	case NodeKindClient:
		i.Dataplane = ""
	case NodeKindDataplane:
		i.Envoy = ""
	default:
		// do nothing
	}
	return i
}

// ChooseConsul controls which image assigns to Consul
func (i Images) ChooseConsul(enterprise bool) Images {
	if enterprise {
		i.Consul = i.ConsulEnterprise
	} else {
		i.Consul = i.ConsulCE
	}
	i.ConsulEnterprise = ""
	i.ConsulCE = ""

	// extract the version part of Consul
	i.consulVersion = i.Consul[strings.Index(i.Consul, ":")+1:]
	return i
}

// GreaterThanVersion compares the image version to a specified version
func (i Images) GreaterThanVersion(version *goversion.Version) bool {
	if i.consulVersion == "local" {
		return true
	}
	iVer := goversion.Must(goversion.NewVersion(i.consulVersion))
	return iVer.GreaterThanOrEqual(version)
}

func (i Images) OverrideWith(i2 Images) Images {
	if i2.Consul != "" {
		i.Consul = i2.Consul
	}
	if i2.ConsulCE != "" {
		i.ConsulCE = i2.ConsulCE
	}
	if i2.ConsulEnterprise != "" {
		i.ConsulEnterprise = i2.ConsulEnterprise
	}
	if i2.Envoy != "" {
		i.Envoy = i2.Envoy
	}
	if i2.Dataplane != "" {
		i.Dataplane = i2.Dataplane
	}
	return i
}

// DefaultImages controls which specific docker images are used as default
// values for topology components that do not specify values.
//
// These can be bulk-updated using the make target 'make update-defaults'
func DefaultImages() Images {
	return Images{
		Consul:           "",
		ConsulCE:         DefaultConsulCEImage,
		ConsulEnterprise: DefaultConsulEnterpriseImage,
		Envoy:            DefaultEnvoyImage,
		Dataplane:        DefaultDataplaneImage,
	}
}
