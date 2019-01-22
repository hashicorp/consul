package services

import (
	"reflect"
	"time"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/mapstructure"
)

// ServicesFromFiles returns the list of agent service registration structs
// from a set of file arguments.
func ServicesFromFiles(files []string) ([]*api.AgentServiceRegistration, error) {
	// We set devMode to true so we can get the basic valid default
	// configuration. devMode doesn't set any services by default so this
	// is okay since we only look at services.
	devMode := true
	b, err := config.NewBuilder(config.Flags{
		ConfigFiles: files,
		DevMode:     &devMode,
	})
	if err != nil {
		return nil, err
	}

	cfg, err := b.BuildAndValidate()
	if err != nil {
		return nil, err
	}

	// The services are now in "structs.ServiceDefinition" form and we need
	// them in "api.AgentServiceRegistration" form so do the conversion.
	result := make([]*api.AgentServiceRegistration, 0, len(cfg.Services))
	for _, svc := range cfg.Services {
		apiSvc, err := serviceToAgentService(svc)
		if err != nil {
			return nil, err
		}

		result = append(result, apiSvc)
	}

	return result, nil
}

// serviceToAgentService converts a ServiceDefinition struct to an
// AgentServiceRegistration API struct.
func serviceToAgentService(svc *structs.ServiceDefinition) (*api.AgentServiceRegistration, error) {
	// mapstructure can do this for us, but we encapsulate it in this
	// helper function in case we need to change the logic in the future.
	var result api.AgentServiceRegistration
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &result,
		DecodeHook:       timeDurationToStringHookFunc(),
		WeaklyTypedInput: true,
	})
	if err != nil {
		return nil, err
	}
	if err := d.Decode(svc); err != nil {
		return nil, err
	}

	// The structs version has non-pointer checks and the destination
	// has pointers, so we need to set the destination to nil if there
	// is no check ID set.
	if result.Check != nil && result.Check.Name == "" {
		result.Check = nil
	}
	if len(result.Checks) == 1 && result.Checks[0].Name == "" {
		result.Checks = nil
	}

	return &result, nil
}

// timeDurationToStringHookFunc returns a DecodeHookFunc that converts
// time.Duration to string.
func timeDurationToStringHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		dur, ok := data.(time.Duration)
		if !ok {
			return data, nil
		}
		if t.Kind() != reflect.String {
			return data, nil
		}
		if dur == 0 {
			return "", nil
		}

		// Convert it by parsing
		return data.(time.Duration).String(), nil
	}
}
