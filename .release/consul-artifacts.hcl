# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

schema = 1
artifacts {
  zip = [
    "consul_${version}_darwin_amd64.zip",
    "consul_${version}_darwin_arm64.zip",
    "consul_${version}_freebsd_386.zip",
    "consul_${version}_freebsd_amd64.zip",
    "consul_${version}_linux_386.zip",
    "consul_${version}_linux_amd64.zip",
    "consul_${version}_linux_arm.zip",
    "consul_${version}_linux_arm64.zip",
    "consul_${version}_solaris_amd64.zip",
    "consul_${version}_windows_386.zip",
    "consul_${version}_windows_amd64.zip",
  ]
  rpm = [
    "consul-${version_linux}-1.aarch64.rpm",
    "consul-${version_linux}-1.armv7hl.rpm",
    "consul-${version_linux}-1.i386.rpm",
    "consul-${version_linux}-1.x86_64.rpm",
  ]
  deb = [
    "consul_${version_linux}-1_amd64.deb",
    "consul_${version_linux}-1_arm64.deb",
    "consul_${version_linux}-1_armhf.deb",
    "consul_${version_linux}-1_i386.deb",
  ]
  container = [
    "consul_default_linux_386_${version}_${commit_sha}.docker.dev.tar",
    "consul_default_linux_386_${version}_${commit_sha}.docker.tar",
    "consul_default_linux_amd64_${version}_${commit_sha}.docker.dev.tar",
    "consul_default_linux_amd64_${version}_${commit_sha}.docker.tar",
    "consul_default_linux_arm64_${version}_${commit_sha}.docker.dev.tar",
    "consul_default_linux_arm64_${version}_${commit_sha}.docker.tar",
    "consul_default_linux_arm_${version}_${commit_sha}.docker.dev.tar",
    "consul_default_linux_arm_${version}_${commit_sha}.docker.tar",
    "consul_ubi_linux_amd64_${version}_${commit_sha}.docker.dev.tar",
    "consul_ubi_linux_amd64_${version}_${commit_sha}.docker.redhat.tar",
    "consul_ubi_linux_amd64_${version}_${commit_sha}.docker.tar",
  ]
}
