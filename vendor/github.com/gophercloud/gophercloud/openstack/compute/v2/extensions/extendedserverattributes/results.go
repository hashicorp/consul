package extendedserverattributes

// ServerAttributesExt represents OS-EXT-SRV-ATTR server response fields.
//
// Following fields will be added after implementing full API microversion
// support in the Gophercloud:
//
//  - OS-EXT-SRV-ATTR:reservation_id"
//  - OS-EXT-SRV-ATTR:launch_index"
//  - OS-EXT-SRV-ATTR:hostname"
//  - OS-EXT-SRV-ATTR:kernel_id"
//  - OS-EXT-SRV-ATTR:ramdisk_id"
//  - OS-EXT-SRV-ATTR:root_device_name"
//  - OS-EXT-SRV-ATTR:user_data"
type ServerAttributesExt struct {
	Host               string `json:"OS-EXT-SRV-ATTR:host"`
	InstanceName       string `json:"OS-EXT-SRV-ATTR:instance_name"`
	HypervisorHostname string `json:"OS-EXT-SRV-ATTR:hypervisor_hostname"`
}
