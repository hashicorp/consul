// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tfgen

import (
	"fmt"
	"regexp"
)

var invalidResourceName = regexp.MustCompile(`[^a-z0-9-]+`)

func DockerImageResourceName(image string) string {
	return invalidResourceName.ReplaceAllLiteralString(image, "-")
}

func DockerNetwork(name, subnet string) Resource {
	return Text(fmt.Sprintf(`
resource "docker_network" %[1]q {
  name       = %[1]q
  attachable = true
  ipam_config {
    subnet = %[2]q
  }
}
`, name, subnet))
}

func DockerVolume(name string) Resource {
	return Text(fmt.Sprintf(`
resource "docker_volume" %[1]q {
  name       = %[1]q
}`, name))
}

func DockerImage(name, image string) Resource {
	return Text(fmt.Sprintf(`
resource "docker_image" %[1]q {
  name = %[2]q
  keep_locally = true
}`, name, image))
}
