// +build linux

package host

import (
	"testing"
)

func TestGetRedhatishVersion(t *testing.T) {
	var ret string
	c := []string{"Rawhide"}
	ret = getRedhatishVersion(c)
	if ret != "rawhide" {
		t.Errorf("Could not get version rawhide: %v", ret)
	}

	c = []string{"Fedora release 15 (Lovelock)"}
	ret = getRedhatishVersion(c)
	if ret != "15" {
		t.Errorf("Could not get version fedora: %v", ret)
	}

	c = []string{"Enterprise Linux Server release 5.5 (Carthage)"}
	ret = getRedhatishVersion(c)
	if ret != "5.5" {
		t.Errorf("Could not get version redhat enterprise: %v", ret)
	}

	c = []string{""}
	ret = getRedhatishVersion(c)
	if ret != "" {
		t.Errorf("Could not get version with no value: %v", ret)
	}
}

func TestGetRedhatishPlatform(t *testing.T) {
	var ret string
	c := []string{"red hat"}
	ret = getRedhatishPlatform(c)
	if ret != "redhat" {
		t.Errorf("Could not get platform redhat: %v", ret)
	}

	c = []string{"Fedora release 15 (Lovelock)"}
	ret = getRedhatishPlatform(c)
	if ret != "fedora" {
		t.Errorf("Could not get platform fedora: %v", ret)
	}

	c = []string{"Enterprise Linux Server release 5.5 (Carthage)"}
	ret = getRedhatishPlatform(c)
	if ret != "enterprise" {
		t.Errorf("Could not get platform redhat enterprise: %v", ret)
	}

	c = []string{""}
	ret = getRedhatishPlatform(c)
	if ret != "" {
		t.Errorf("Could not get platform with no value: %v", ret)
	}
}
