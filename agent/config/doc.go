// Package config contains the command line and config file code for the
// consul agent.
//
// The consul agent configuration is generated from multiple sources:
//
//  * config files
//  * environment variables (which?)
//  * cmd line args
//
// Each of these argument sets needs to be parsed, validated and then
// merged with the other sources to build the final configuration.
//
// This patch introduces a distinction between the user and the runtime
// configuration. The user configuration defines the external interface for
// the user, i.e. the command line flags, the environment variables and the
// config file format which cannot be changed without breaking the users'
// setup.
//
// The runtime configuration is the merged, validated and mangled
// configuration structure suitable for the consul agent. Both structures
// are similar but different and the runtime configuration can be
// refactored at will without affecting the user configuration format.
//
// For this, the user configuration consists of several structures for
// config files and command line arguments. Again, the config file and
// command line structs are similar but not identical for historical
// reasons and to allow evolving them differently.
//
// All of the user configuration structs have pointer values to
// unambiguously merge values from several sources into the final value.
//
// The runtime configuration has no pointer values and should be passed by
// value to avoid accidental or malicious runtime configuration changes.
// Runtime updates need to be handled through a new configuration
// instances.

// # Removed command line flags
//
//  * "-atlas" is deprecated and is no longer used. Please remove it from your configuration.
//  * "-atlas-token" is deprecated and is no longer used. Please remove it from your configuration.
//  * "-atlas-join" is deprecated and is no longer used. Please remove it from your configuration.
//  * "-atlas-endpoint" is deprecated and is no longer used. Please remove it from your configuration.
//  * "-dc" is deprecated. Please use "-datacenter" instead
//  * "-retry-join-azure-tag-name" is deprecated. Please use "-retry-join" instead.
//  * "-retry-join-azure-tag-value" is deprecated. Please use "-retry-join" instead.
//  * "-retry-join-ec2-region" is deprecated. Please use "-retry-join" instead.
//  * "-retry-join-ec2-tag-key" is deprecated. Please use "-retry-join" instead.
//  * "-retry-join-ec2-tag-value" is deprecated. Please use "-retry-join" instead.
//  * "-retry-join-gce-credentials-file" is deprecated. Please use "-retry-join" instead.
//  * "-retry-join-gce-project-name" is deprecated. Please use "-retry-join" instead.
//  * "-retry-join-gce-tag-name" is deprecated. Please use "-retry-join" instead.
//  * "-retry-join-gce-zone-pattern" is deprecated. Please use "-retry-join" instead.
//
// # Removed configuration fields
//
// 	* "addresses.rpc" is deprecated and is no longer used. Please remove it from your configuration.
// 	* "ports.rpc" is deprecated and is no longer used. Please remove it from your configuration.
// 	* "atlas_infrastructure" is deprecated and is no longer used. Please remove it from your configuration.
// 	* "atlas_token" is deprecated and is no longer used. Please remove it from your configuration.
// 	* "atlas_acl_token" is deprecated and is no longer used. Please remove it from your configuration.
// 	* "atlas_join" is deprecated and is no longer used. Please remove it from your configuration.
// 	* "atlas_endpoint" is deprecated and is no longer used. Please remove it from your configuration.
// 	* "http_api_response_headers" is deprecated. Please use "http_config.response_headers" instead.
// 	* "dogstatsd_addr" is deprecated. Please use "telemetry.dogstatsd_addr" instead.
// 	* "dogstatsd_tags" is deprecated. Please use "telemetry.dogstatsd_tags" instead.
// 	* "recursor" is deprecated. Please use "recursors" instead.
// 	* "statsd_addr" is deprecated. Please use "telemetry.statsd_addr" instead.
// 	* "statsite_addr" is deprecated. Please use "telemetry.statsite_addr" instead.
// 	* "statsite_prefix" is deprecated. Please use "telemetry.metrics_prefix" instead.
// 	* "telemetry.statsite_prefix" is deprecated. Please use "telemetry.metrics_prefix" instead.
//  * "retry_join_azure" is deprecated. Please use "retry_join" instead.
//  * "retry_join_ec2" is deprecated. Please use "retry_join" instead.
//  * "retry_join_gce" is deprecated. Please use "retry_join" instead.
//
// # Removed service config alias fields
//
//  * "serviceid" is deprecated in service definitions. Please use "service_id" instead.
//  * "dockercontainerid" is deprecated in service definitions. Please use "docker_container_id" instead.
//  * "tlsskipverify" is deprecated in service definitions. Please use "tls_skip_verify" instead.
//  * "deregistercriticalserviceafter" is deprecated in service definitions. Please use "deregister_critical_service_after" instead.

package config
