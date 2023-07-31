package accesslogs

import (
	"fmt"

	envoy_accesslog_v3 "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_fileaccesslog_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	envoy_streamaccesslog_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/stream/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

const (
	defaultJSONFormat = `
{
	"start_time":                        "%START_TIME%",
	"route_name":                        "%ROUTE_NAME%",
	"method":                            "%REQ(:METHOD)%",
	"path":                              "%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%",
	"protocol":                          "%PROTOCOL%",
	"response_code":                     "%RESPONSE_CODE%",
	"response_flags":                    "%RESPONSE_FLAGS%",
	"response_code_details":             "%RESPONSE_CODE_DETAILS%",
	"connection_termination_details":    "%CONNECTION_TERMINATION_DETAILS%",
	"bytes_received":                    "%BYTES_RECEIVED%",
	"bytes_sent":                        "%BYTES_SENT%",
	"duration":                          "%DURATION%",
	"upstream_service_time":             "%RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%",
	"x_forwarded_for":                   "%REQ(X-FORWARDED-FOR)%",
	"user_agent":                        "%REQ(USER-AGENT)%",
	"request_id":                        "%REQ(X-REQUEST-ID)%",
	"authority":                         "%REQ(:AUTHORITY)%",
	"upstream_host":                     "%UPSTREAM_HOST%",
	"upstream_cluster":                  "%UPSTREAM_CLUSTER%",
	"upstream_local_address":            "%UPSTREAM_LOCAL_ADDRESS%",
	"downstream_local_address":          "%DOWNSTREAM_LOCAL_ADDRESS%",
	"downstream_remote_address":         "%DOWNSTREAM_REMOTE_ADDRESS%",
	"requested_server_name":             "%REQUESTED_SERVER_NAME%",
	"upstream_transport_failure_reason": "%UPSTREAM_TRANSPORT_FAILURE_REASON%"
}
`
)

// MakeAccessLogs returns a fully-hydrated slice of Envoy Access log configurations based
// on the proxy-defaults settings. Currently only one access logger is supported.
// Listeners (as opposed to listener filters) can trigger an access log filter with the boolean.
// Tests are located in agent/xds/listeners_test.go.
func MakeAccessLogs(logs *structs.AccessLogsConfig, isListener bool) ([]*envoy_accesslog_v3.AccessLog, error) {
	if logs == nil || !logs.Enabled {
		return nil, nil
	}

	if isListener && logs.DisableListenerLogs {
		return nil, nil
	}

	config, err := getLogger(logs)
	if err != nil {
		return nil, fmt.Errorf("failed to get logger: %w", err)
	}

	var filter *envoy_accesslog_v3.AccessLogFilter
	name := "Consul Listener Filter Log"
	if isListener {
		name = "Consul Listener Log"
		filter = getListenerAccessLogFilter()
	}

	newFilter := &envoy_accesslog_v3.AccessLog{
		Name:   name,
		Filter: filter,
		ConfigType: &envoy_accesslog_v3.AccessLog_TypedConfig{
			TypedConfig: config,
		},
	}

	return []*envoy_accesslog_v3.AccessLog{newFilter}, nil
}

// getLogger returns an individual instance of an Envoy logger based on proxy-defaults
func getLogger(logs *structs.AccessLogsConfig) (*anypb.Any, error) {
	logFormat, err := getLogFormat(logs)
	if err != nil {
		return nil, fmt.Errorf("could not get envoy log format: %w", err)
	}

	switch logs.Type {
	case structs.DefaultLogSinkType, structs.StdOutLogSinkType:
		return getStdoutLogger(logFormat)
	case structs.StdErrLogSinkType:
		return getStderrLogger(logFormat)
	case structs.FileLogSinkType:
		return getFileLogger(logFormat, logs.Path)
	default:
		return nil, fmt.Errorf("unsupported log format: %s", logs.Type)
	}
}

// getLogFormat returns an Envoy log format object that is compatible with all log sinks.
// If a format is not provided in the proxy-defaults, the default JSON format is used.
func getLogFormat(logs *structs.AccessLogsConfig) (*envoy_core_v3.SubstitutionFormatString, error) {

	var format, formatType string
	if logs.TextFormat == "" && logs.JSONFormat == "" {
		format = defaultJSONFormat
		formatType = "json"
	} else if logs.JSONFormat != "" {
		format = logs.JSONFormat
		formatType = "json"
	} else {
		format = logs.TextFormat
		formatType = "text"
	}

	switch formatType {
	case "json":
		jsonFormat := structpb.Struct{}
		if err := jsonFormat.UnmarshalJSON([]byte(format)); err != nil {
			return nil, fmt.Errorf("could not unmarshal JSON format string: %w", err)
		}

		return &envoy_core_v3.SubstitutionFormatString{
			Format: &envoy_core_v3.SubstitutionFormatString_JsonFormat{
				JsonFormat: &jsonFormat,
			},
		}, nil
	case "text":
		textFormat := lib.EnsureTrailingNewline(format)
		return &envoy_core_v3.SubstitutionFormatString{
			Format: &envoy_core_v3.SubstitutionFormatString_TextFormatSource{
				TextFormatSource: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineString{
						InlineString: textFormat,
					},
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("invalid log format type")
	}
}

// getStdoutLogger returns Envoy's representation of a stdout log sink with the provided format.
func getStdoutLogger(logFormat *envoy_core_v3.SubstitutionFormatString) (*anypb.Any, error) {
	return anypb.New(&envoy_streamaccesslog_v3.StdoutAccessLog{
		AccessLogFormat: &envoy_streamaccesslog_v3.StdoutAccessLog_LogFormat{
			LogFormat: logFormat,
		},
	})
}

// getStderrLogger returns Envoy's representation of a stderr log sink with the provided format.
func getStderrLogger(logFormat *envoy_core_v3.SubstitutionFormatString) (*anypb.Any, error) {
	return anypb.New(&envoy_streamaccesslog_v3.StderrAccessLog{
		AccessLogFormat: &envoy_streamaccesslog_v3.StderrAccessLog_LogFormat{
			LogFormat: logFormat,
		},
	})
}

// getFileLogger returns Envoy's representation of a file log sink with the provided format and path to a file.
func getFileLogger(logFormat *envoy_core_v3.SubstitutionFormatString, path string) (*anypb.Any, error) {
	return anypb.New(&envoy_fileaccesslog_v3.FileAccessLog{
		AccessLogFormat: &envoy_fileaccesslog_v3.FileAccessLog_LogFormat{
			LogFormat: logFormat,
		},
		Path: path,
	})
}

// getListenerAccessLogFilter returns a filter that will be used on listeners to decide when a log is emitted.
// Set to "NR" which corresponds to "No route configured for a given request in addition
// to 404 response code, or no matching filter chain for a downstream connection."
func getListenerAccessLogFilter() *envoy_accesslog_v3.AccessLogFilter {
	return &envoy_accesslog_v3.AccessLogFilter{
		FilterSpecifier: &envoy_accesslog_v3.AccessLogFilter_ResponseFlagFilter{
			ResponseFlagFilter: &envoy_accesslog_v3.ResponseFlagFilter{Flags: []string{"NR"}},
		},
	}
}
