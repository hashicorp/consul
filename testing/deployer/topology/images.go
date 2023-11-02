// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topology

import (
	"strings"
)

type Images struct {
	// Consul is the image used for creating the container,
	// Use ChooseConsul() to control which image (ConsulCE or ConsulEnterprise) assign to Consul
	Consul string `json:",omitempty"`
	// ConsulCE sets the CE image
	ConsulCE string `json:",omitempty"`
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

	repo, name, ok := strings.Cut(img, "/")
	if ok {
		name = repo + "-" + name
	}

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

	repo1, name1, ok1 := strings.Cut(img1, "/")
	repo2, name2, ok2 := strings.Cut(img2, "/")

	if ok1 {
		name1 = repo1 + "-" + name1
	} else {
		name1 = repo1
	}
	if ok2 {
		name2 = repo2 + "-" + name2
	} else {
		name2 = repo2
	}

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
	return i
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
		ConsulCE:         DefaultConsulImage,
		ConsulEnterprise: DefaultConsulEnterpriseImage,
		Envoy:            DefaultEnvoyImage,
		Dataplane:        DefaultDataplaneImage,
	}
}
