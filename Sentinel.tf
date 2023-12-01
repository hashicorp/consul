# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

policy "ensure-vm-disks-for-critical-vms-are-encrypted-with-customer-supplied-encryption-keys" {
  source = "./policies/ensure-vm-disks-for-critical-vms-are-encrypted-with-customer-supplied-encryption-keys/ensure-vm-disks-for-critical-vms-are-encrypted-with-customer-supplied-encryption-keys.sentinel"
}
policy "ensure-oslogin-is-enabled-for-a-project" {
  source = "./policies/ensure-oslogin-is-enabled-for-a-project/ensure-oslogin-is-enabled-for-a-project.sentinel"
}
policy "enable-connecting-to-serial-ports-is-not-enabled-for-vm-instance" {
  source = "./policies/enable-connecting-to-serial-ports-is-not-enabled-for-vm-instance/enable-connecting-to-serial-ports-is-not-enabled-for-vm-instance.sentinel"
}
policy "block-project-wide-ssh-keys-enabled-for-vm-instances" {
  source = "./policies/block-project-wide-ssh-keys-enabled-for-vm-instances/block-project-wide-ssh-keys-enabled-for-vm-instances.sentinel"
}
policy "ensure-that-ip-forwarding-is-not-enabled-on-instances" {
  source = "./policies/ensure-that-ip-forwarding-is-not-enabled-on-instances/ensure-that-ip-forwarding-is-not-enabled-on-instances.sentinel"
}
