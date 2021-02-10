package services

import (
	"reflect"
	"time"

	"github.com/mitchellh/cli"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

// ServicesFromFiles returns the list of agent service registration structs
// from a set of file arguments.
func ServicesFromFiles(ui cli.Ui, files []string) ([]*api.AgentServiceRegistration, error) {
	// We set devMode to true so we can get the basic valid default
	// configuration. devMode doesn't set any services by default so this
	// is okay since we only look at services.
	devMode := true
	r, err := config.Load(config.LoadOpts{
		ConfigFiles: files,
		DevMode:     &devMode,
	})
	if err != nil {
		return nil, err
	}
	for _, w := range r.Warnings {
		ui.Warn(w)
	}

	// The services are now in "structs.ServiceDefinition" form and we need
	// them in "api.AgentServiceRegistration" form so do the conversion.
	services := r.RuntimeConfig.Services
	result := make([]*api.AgentServiceRegistration, 0, len(services))
	for _, svc := range services {
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
	// is a zero-value Check field.
	if result.Check != nil && reflect.DeepEqual(*result.Check, api.AgentServiceCheck{}) {
		result.Check = nil
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
