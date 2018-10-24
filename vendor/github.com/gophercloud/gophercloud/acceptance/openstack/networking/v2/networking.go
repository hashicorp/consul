package v2

import (
	"fmt"
	"testing"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/extradhcpopts"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/portsecurity"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
)

// CreateNetwork will create basic network. An error will be returned if the
// network could not be created.
func CreateNetwork(t *testing.T, client *gophercloud.ServiceClient) (*networks.Network, error) {
	networkName := tools.RandomString("TESTACC-", 8)
	createOpts := networks.CreateOpts{
		Name:         networkName,
		AdminStateUp: gophercloud.Enabled,
	}

	t.Logf("Attempting to create network: %s", networkName)

	network, err := networks.Create(client, createOpts).Extract()
	if err != nil {
		return network, err
	}

	t.Logf("Successfully created network.")
	return network, nil
}

// CreateNetworkWithoutPortSecurity will create a network without port security.
// An error will be returned if the network could not be created.
func CreateNetworkWithoutPortSecurity(t *testing.T, client *gophercloud.ServiceClient) (*networks.Network, error) {
	networkName := tools.RandomString("TESTACC-", 8)
	networkCreateOpts := networks.CreateOpts{
		Name:         networkName,
		AdminStateUp: gophercloud.Enabled,
	}

	iFalse := false
	createOpts := portsecurity.NetworkCreateOptsExt{
		CreateOptsBuilder:   networkCreateOpts,
		PortSecurityEnabled: &iFalse,
	}

	t.Logf("Attempting to create network: %s", networkName)

	network, err := networks.Create(client, createOpts).Extract()
	if err != nil {
		return network, err
	}

	t.Logf("Successfully created network.")
	return network, nil
}

// CreatePort will create a port on the specified subnet. An error will be
// returned if the port could not be created.
func CreatePort(t *testing.T, client *gophercloud.ServiceClient, networkID, subnetID string) (*ports.Port, error) {
	portName := tools.RandomString("TESTACC-", 8)

	t.Logf("Attempting to create port: %s", portName)

	createOpts := ports.CreateOpts{
		NetworkID:    networkID,
		Name:         portName,
		AdminStateUp: gophercloud.Enabled,
		FixedIPs:     []ports.IP{ports.IP{SubnetID: subnetID}},
	}

	port, err := ports.Create(client, createOpts).Extract()
	if err != nil {
		return port, err
	}

	if err := WaitForPortToCreate(client, port.ID, 60); err != nil {
		return port, err
	}

	newPort, err := ports.Get(client, port.ID).Extract()
	if err != nil {
		return newPort, err
	}

	t.Logf("Successfully created port: %s", portName)

	return newPort, nil
}

// CreatePortWithNoSecurityGroup will create a port with no security group
// attached. An error will be returned if the port could not be created.
func CreatePortWithNoSecurityGroup(t *testing.T, client *gophercloud.ServiceClient, networkID, subnetID string) (*ports.Port, error) {
	portName := tools.RandomString("TESTACC-", 8)
	iFalse := false

	t.Logf("Attempting to create port: %s", portName)

	createOpts := ports.CreateOpts{
		NetworkID:      networkID,
		Name:           portName,
		AdminStateUp:   &iFalse,
		FixedIPs:       []ports.IP{ports.IP{SubnetID: subnetID}},
		SecurityGroups: &[]string{},
	}

	port, err := ports.Create(client, createOpts).Extract()
	if err != nil {
		return port, err
	}

	if err := WaitForPortToCreate(client, port.ID, 60); err != nil {
		return port, err
	}

	newPort, err := ports.Get(client, port.ID).Extract()
	if err != nil {
		return newPort, err
	}

	t.Logf("Successfully created port: %s", portName)

	return newPort, nil
}

// CreatePortWithoutPortSecurity will create a port without port security on the
// specified subnet. An error will be returned if the port could not be created.
func CreatePortWithoutPortSecurity(t *testing.T, client *gophercloud.ServiceClient, networkID, subnetID string) (*ports.Port, error) {
	portName := tools.RandomString("TESTACC-", 8)

	t.Logf("Attempting to create port: %s", portName)

	portCreateOpts := ports.CreateOpts{
		NetworkID:    networkID,
		Name:         portName,
		AdminStateUp: gophercloud.Enabled,
		FixedIPs:     []ports.IP{ports.IP{SubnetID: subnetID}},
	}

	iFalse := false
	createOpts := portsecurity.PortCreateOptsExt{
		CreateOptsBuilder:   portCreateOpts,
		PortSecurityEnabled: &iFalse,
	}

	port, err := ports.Create(client, createOpts).Extract()
	if err != nil {
		return port, err
	}

	if err := WaitForPortToCreate(client, port.ID, 60); err != nil {
		return port, err
	}

	newPort, err := ports.Get(client, port.ID).Extract()
	if err != nil {
		return newPort, err
	}

	t.Logf("Successfully created port: %s", portName)

	return newPort, nil
}

// CreateSubnet will create a subnet on the specified Network ID. An error
// will be returned if the subnet could not be created.
func CreateSubnet(t *testing.T, client *gophercloud.ServiceClient, networkID string) (*subnets.Subnet, error) {
	subnetName := tools.RandomString("TESTACC-", 8)
	subnetOctet := tools.RandomInt(1, 250)
	subnetCIDR := fmt.Sprintf("192.168.%d.0/24", subnetOctet)
	subnetGateway := fmt.Sprintf("192.168.%d.1", subnetOctet)
	createOpts := subnets.CreateOpts{
		NetworkID:  networkID,
		CIDR:       subnetCIDR,
		IPVersion:  4,
		Name:       subnetName,
		EnableDHCP: gophercloud.Disabled,
		GatewayIP:  &subnetGateway,
	}

	t.Logf("Attempting to create subnet: %s", subnetName)

	subnet, err := subnets.Create(client, createOpts).Extract()
	if err != nil {
		return subnet, err
	}

	t.Logf("Successfully created subnet.")
	return subnet, nil
}

// CreateSubnetWithDefaultGateway will create a subnet on the specified Network
// ID and have Neutron set the gateway by default An error will be returned if
// the subnet could not be created.
func CreateSubnetWithDefaultGateway(t *testing.T, client *gophercloud.ServiceClient, networkID string) (*subnets.Subnet, error) {
	subnetName := tools.RandomString("TESTACC-", 8)
	subnetOctet := tools.RandomInt(1, 250)
	subnetCIDR := fmt.Sprintf("192.168.%d.0/24", subnetOctet)
	createOpts := subnets.CreateOpts{
		NetworkID:  networkID,
		CIDR:       subnetCIDR,
		IPVersion:  4,
		Name:       subnetName,
		EnableDHCP: gophercloud.Disabled,
	}

	t.Logf("Attempting to create subnet: %s", subnetName)

	subnet, err := subnets.Create(client, createOpts).Extract()
	if err != nil {
		return subnet, err
	}

	t.Logf("Successfully created subnet.")
	return subnet, nil
}

// CreateSubnetWithNoGateway will create a subnet with no gateway on the
// specified Network ID.  An error will be returned if the subnet could not be
// created.
func CreateSubnetWithNoGateway(t *testing.T, client *gophercloud.ServiceClient, networkID string) (*subnets.Subnet, error) {
	var noGateway = ""
	subnetName := tools.RandomString("TESTACC-", 8)
	subnetOctet := tools.RandomInt(1, 250)
	subnetCIDR := fmt.Sprintf("192.168.%d.0/24", subnetOctet)
	dhcpStart := fmt.Sprintf("192.168.%d.10", subnetOctet)
	dhcpEnd := fmt.Sprintf("192.168.%d.200", subnetOctet)
	createOpts := subnets.CreateOpts{
		NetworkID:  networkID,
		CIDR:       subnetCIDR,
		IPVersion:  4,
		Name:       subnetName,
		EnableDHCP: gophercloud.Disabled,
		GatewayIP:  &noGateway,
		AllocationPools: []subnets.AllocationPool{
			{
				Start: dhcpStart,
				End:   dhcpEnd,
			},
		},
	}

	t.Logf("Attempting to create subnet: %s", subnetName)

	subnet, err := subnets.Create(client, createOpts).Extract()
	if err != nil {
		return subnet, err
	}

	t.Logf("Successfully created subnet.")
	return subnet, nil
}

// CreateSubnetWithSubnetPool will create a subnet associated with the provided subnetpool on the specified Network ID.
// An error will be returned if the subnet or the subnetpool could not be created.
func CreateSubnetWithSubnetPool(t *testing.T, client *gophercloud.ServiceClient, networkID string, subnetPoolID string) (*subnets.Subnet, error) {
	subnetName := tools.RandomString("TESTACC-", 8)
	subnetOctet := tools.RandomInt(1, 250)
	subnetCIDR := fmt.Sprintf("10.%d.0.0/24", subnetOctet)
	createOpts := subnets.CreateOpts{
		NetworkID:    networkID,
		CIDR:         subnetCIDR,
		IPVersion:    4,
		Name:         subnetName,
		EnableDHCP:   gophercloud.Disabled,
		SubnetPoolID: subnetPoolID,
	}

	t.Logf("Attempting to create subnet: %s", subnetName)

	subnet, err := subnets.Create(client, createOpts).Extract()
	if err != nil {
		return subnet, err
	}

	t.Logf("Successfully created subnet.")
	return subnet, nil
}

// CreateSubnetWithSubnetPoolNoCIDR will create a subnet associated with the
// provided subnetpool on the specified Network ID.
// An error will be returned if the subnet or the subnetpool could not be created.
func CreateSubnetWithSubnetPoolNoCIDR(t *testing.T, client *gophercloud.ServiceClient, networkID string, subnetPoolID string) (*subnets.Subnet, error) {
	subnetName := tools.RandomString("TESTACC-", 8)
	createOpts := subnets.CreateOpts{
		NetworkID:    networkID,
		IPVersion:    4,
		Name:         subnetName,
		EnableDHCP:   gophercloud.Disabled,
		SubnetPoolID: subnetPoolID,
	}

	t.Logf("Attempting to create subnet: %s", subnetName)

	subnet, err := subnets.Create(client, createOpts).Extract()
	if err != nil {
		return subnet, err
	}

	t.Logf("Successfully created subnet.")
	return subnet, nil
}

// DeleteNetwork will delete a network with a specified ID. A fatal error will
// occur if the delete was not successful. This works best when used as a
// deferred function.
func DeleteNetwork(t *testing.T, client *gophercloud.ServiceClient, networkID string) {
	t.Logf("Attempting to delete network: %s", networkID)

	err := networks.Delete(client, networkID).ExtractErr()
	if err != nil {
		t.Fatalf("Unable to delete network %s: %v", networkID, err)
	}

	t.Logf("Deleted network: %s", networkID)
}

// DeletePort will delete a port with a specified ID. A fatal error will
// occur if the delete was not successful. This works best when used as a
// deferred function.
func DeletePort(t *testing.T, client *gophercloud.ServiceClient, portID string) {
	t.Logf("Attempting to delete port: %s", portID)

	err := ports.Delete(client, portID).ExtractErr()
	if err != nil {
		t.Fatalf("Unable to delete port %s: %v", portID, err)
	}

	t.Logf("Deleted port: %s", portID)
}

// DeleteSubnet will delete a subnet with a specified ID. A fatal error will
// occur if the delete was not successful. This works best when used as a
// deferred function.
func DeleteSubnet(t *testing.T, client *gophercloud.ServiceClient, subnetID string) {
	t.Logf("Attempting to delete subnet: %s", subnetID)

	err := subnets.Delete(client, subnetID).ExtractErr()
	if err != nil {
		t.Fatalf("Unable to delete subnet %s: %v", subnetID, err)
	}

	t.Logf("Deleted subnet: %s", subnetID)
}

func WaitForPortToCreate(client *gophercloud.ServiceClient, portID string, secs int) error {
	return gophercloud.WaitFor(secs, func() (bool, error) {
		p, err := ports.Get(client, portID).Extract()
		if err != nil {
			return false, err
		}

		if p.Status == "ACTIVE" || p.Status == "DOWN" {
			return true, nil
		}

		return false, nil
	})
}

// PortWithExtraDHCPOpts represents a port with extra DHCP options configuration.
type PortWithExtraDHCPOpts struct {
	ports.Port
	extradhcpopts.ExtraDHCPOptsExt
}

// CreatePortWithExtraDHCPOpts will create a port with DHCP options on the
// specified subnet. An error will be returned if the port could not be created.
func CreatePortWithExtraDHCPOpts(t *testing.T, client *gophercloud.ServiceClient, networkID, subnetID string) (*PortWithExtraDHCPOpts, error) {
	portName := tools.RandomString("TESTACC-", 8)

	t.Logf("Attempting to create port: %s", portName)

	portCreateOpts := ports.CreateOpts{
		NetworkID:    networkID,
		Name:         portName,
		AdminStateUp: gophercloud.Enabled,
		FixedIPs:     []ports.IP{ports.IP{SubnetID: subnetID}},
	}

	createOpts := extradhcpopts.CreateOptsExt{
		CreateOptsBuilder: portCreateOpts,
		ExtraDHCPOpts: []extradhcpopts.CreateExtraDHCPOpt{
			{
				OptName:  "test_option_1",
				OptValue: "test_value_1",
			},
		},
	}
	port := &PortWithExtraDHCPOpts{}

	err := ports.Create(client, createOpts).ExtractInto(port)
	if err != nil {
		return nil, err
	}

	if err := WaitForPortToCreate(client, port.ID, 60); err != nil {
		return nil, err
	}

	err = ports.Get(client, port.ID).ExtractInto(port)
	if err != nil {
		return port, err
	}

	t.Logf("Successfully created port: %s", portName)

	return port, nil
}
