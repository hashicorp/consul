package topology

import (
	"strings"
)

type Images struct {
	Consul           string `json:",omitempty"`
	ConsulOSS        string `json:",omitempty"`
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

func (i Images) EnvoyConsulImage() string {
	if i.Consul == "" || i.Envoy == "" {
		return ""
	}

	img1, tag1, ok1 := strings.Cut(i.Consul, ":")
	img2, tag2, ok2 := strings.Cut(i.Envoy, ":")
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

	// ex: local/hashicorp-consul-and-envoyproxy-envoy:1.15.0-with-v1.26.2
	return "local/" + name1 + "-and-" + name2 + ":" + tag1 + "-with-" + tag2
}

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

func (i Images) ChooseConsul(enterprise bool) Images {
	if enterprise {
		i.Consul = i.ConsulEnterprise
	} else {
		i.Consul = i.ConsulOSS
	}
	i.ConsulEnterprise = ""
	i.ConsulOSS = ""
	return i
}

func (i Images) OverrideWith(i2 Images) Images {
	if i2.Consul != "" {
		i.Consul = i2.Consul
	}
	if i2.ConsulOSS != "" {
		i.ConsulOSS = i2.ConsulOSS
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
		ConsulOSS:        DefaultConsulImage,
		ConsulEnterprise: DefaultConsulEnterpriseImage,
		Envoy:            DefaultEnvoyImage,
		Dataplane:        DefaultDataplaneImage,
	}
}
