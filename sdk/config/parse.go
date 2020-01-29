package config

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/mitchellh/mapstructure"
)

// ParserConfig encompasses all of the configuration necessary to parse some potentially HCL based configuration properly
type ParserConfig struct {
	// SkipPatchSliceOfMaps controls which keys in the raw map[string]interface{}
	// are not considered candidates for merging. Sub-keys will still be patched.
	SkipPatchSliceOfMaps []string
	// SkipPatchSliceOfMapsTree is like the normal variant except that sub-keys will
	// not be patched.
	SkipPatchSliceOfMapsTree []string

	// KeyTranslations is a map of original keys to the desired key name
	KeyTranslations map[string]string
}

var (
	ConsulConfigParser = ParserConfig{
		SkipPatchSliceOfMaps: []string{
			"checks",
			"segments",
			"service.checks",
			"services",
			"services.checks",
			"watches",
			"service.connect.proxy.config.upstreams", // Deprecated
			"services.connect.proxy.config.upstreams", // Deprecated
			"service.connect.proxy.upstreams",
			"services.connect.proxy.upstreams",
			"service.connect.proxy.expose.paths",
			"services.connect.proxy.expose.paths",
			"service.proxy.upstreams",
			"services.proxy.upstreams",
			"service.proxy.expose.paths",
			"services.proxy.expose.paths",

			// Need all the service(s) exceptions also for nested sidecar service.
			"service.connect.sidecar_service.checks",
			"services.connect.sidecar_service.checks",
			"service.connect.sidecar_service.proxy.upstreams",
			"services.connect.sidecar_service.proxy.upstreams",
			"service.connect.sidecar_service.proxy.expose.paths",
			"services.connect.sidecar_service.proxy.expose.paths",

			// need these to account for not snake casing
			"service.connect.SidecarService.checks",
			"services.connect.SidecarService.checks",
			"service.connect.SidecarService.proxy.upstreams",
			"services.connect.SidecarService.proxy.upstreams",
			"service.connect.SidecarService.proxy.expose.paths",
			"services.connect.SidecarService.proxy.expose.paths",
		},
		SkipPatchSliceOfMapsTree: []string{
			"config_entries.bootstrap",
		},
		KeyTranslations: map[string]string{
			"deregistercriticalserviceafter": "deregister_critical_service_after",
			"dockercontainerid":              "docker_container_id",
			"scriptargs":                     "args",
			"serviceid":                      "service_id",
			"tlsskipverify":                  "tls_skip_verify",
			"config_entries.bootstrap":       "",
		},
	}

	ConsulServicesParser = ParserConfig{
		SkipPatchSliceOfMaps: []string{
			"Checks",
			"Service.Checks",
			"Services",
			"Services.Checks",
			"Service.Connect.Proxy.Config.Upstreams", // Deprecated
			"Services.Connect.Proxy.Config.Upstreams", // Deprecated
			"Service.Connect.Proxy.Upstreams",
			"Services.Connect.Proxy.Upstreams",
			"Service.Connect.Proxy.Expose.Paths",
			"Services.Connect.Proxy.Expose.Paths",
			"Service.Proxy.Upstreams",
			"Services.Proxy.Upstreams",
			"Service.Proxy.Expose.Paths",
			"Services.Proxy.Expose.Paths",

			// Need all the Service(s) exceptions also for nested sidecar Service.
			"Service.Connect.SidecarService.Checks",
			"Services.Connect.SidecarService.Checks",
			"Service.Connect.SidecarService.Proxy.Upstreams",
			"Services.Connect.SidecarService.Proxy.Upstreams",
			"Service.Connect.SidecarService.Proxy.Expose.Paths",
			"Services.Connect.SidecarService.Proxy.Expose.Paths",

			// need these too to account for snake casing
			"Service.Connect.sidecar_service.Checks",
			"Services.Connect.sidecar_service.Checks",
			"Service.Connect.sidecar_service.Proxy.Upstreams",
			"Services.Connect.sidecar_service.Proxy.Upstreams",
			"Service.Connect.sidecar_service.Proxy.Expose.Paths",
			"Services.Connect.sidecar_service.Proxy.Expose.Paths",
		},
		KeyTranslations: map[string]string{
			"deregister_critical_service_after": "DeregisterCriticalServiceAfter",
			"docker_container_id":               "DockerContainerID",
			"scriptargs":                        "Args",
			"tls_skip_verify":                   "TLSSkipVerify",
			"grpc_use_tls":                      "GRPCUseTLS",
			"alias_service":                     "AliasService",
			"check_id":                          "CheckID",
			"alias_node":                        "AliasNode",
			"enable_tag_override":               "EnableTagOverride",
			"destination_namespace":             "DestinationNamespace",
			"destination_name":                  "DestinationName",
			"local_bind_address":                "LocalBindAddress",
			"local_bind_port":                   "LocalBindPort",
			"mesh_gateway":                      "MeshGateway",
			"destination_type":                  "DestinationType",
			"parsed_from_check":                 "ParsedFromCheck",
			"listener_port":                     "ListenerPort",
			"local_path_port":                   "LocalPathPort",
			"destination_service_name":          "DestinationServiceName",
			"destination_service_id":            "DestinationServiceID",
			"local_service_address":             "LocalServiceAddress",
			"local_service_port":                "LocalServicePort",
			"sidecar_service":                   "SidecarService",
			"tagged_addresses":                  "TaggedAddresses",
		},
	}
)

// Parse parses a Config Uragment in either JSON or HCL format.
func (c *ParserConfig) Parse(data string, format string, out interface{}) (err error) {
	var raw map[string]interface{}
	switch format {
	case "json":
		err = json.Unmarshal([]byte(data), &raw)
	case "hcl":
		err = hcl.Decode(&raw, data)
	default:
		err = fmt.Errorf("invalid format: %s", format)
	}
	if err != nil {
		return err
	}

	// We want to be able to report fields which we cannot map as an
	// error so that users find typos in their configuration quickly. To
	// achieve this we use the mapstructure library which maps a a raw
	// map[string]interface{} to a nested structure and reports unused
	// fields. The input for a mapstructure.Decode expects a
	// map[string]interface{} as produced by encoding/json.
	//
	// The HCL language allows to repeat map keys which forces it to
	// store nested structs as []map[string]interface{} instead of
	// map[string]interface{}. This is an ambiguity which makes the
	// generated structures incompatible with a corresponding JSON
	// struct. It also does not work well with the mapstructure library.
	//
	// In order to still use the mapstructure library to find unused
	// fields we patch instances of []map[string]interface{} to a
	// map[string]interface{} before we decode that with mapstructure
	//
	m := PatchSliceOfMaps(raw, c.SkipPatchSliceOfMaps, c.SkipPatchSliceOfMapsTree)

	// There is a difference of representation of some fields depending on
	// where they are used. The HTTP API uses CamelCase whereas the config
	// files use snake_case and between the two there is no automatic mapping.
	// While the JSON and HCL parsers match keys without case (both `id` and
	// `ID` are mapped to an ID field) the same thing does not happen between
	// CamelCase and snake_case. Since changing either format would break
	// existing setups we have to support both and slowly transition to one of
	// the formats. Also, there is at least one case where we use the "wrong"
	// key and want to map that to the new key to support deprecation -
	// see [GH-3179]. TranslateKeys maps potentially CamelCased values to the
	// snake_case that is used in the config file parser. If both the CamelCase
	// and snake_case values are set the snake_case value is used and the other
	// value is discarded.
	TranslateKeys(m, c.KeyTranslations)

	var md mapstructure.Metadata
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata: &md,
		Result:   out,
	})
	if err != nil {
		return err
	}
	if err := d.Decode(m); err != nil {
		return err
	}

	for _, k := range md.Unused {
		err = multierror.Append(err, fmt.Errorf("invalid config key %s", k))
	}
	return
}
