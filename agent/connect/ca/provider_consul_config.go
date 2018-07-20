package ca

import (
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/mapstructure"
)

func ParseConsulCAConfig(raw map[string]interface{}) (*structs.ConsulCAProviderConfig, error) {
	config := structs.ConsulCAProviderConfig{
		CommonCAProviderConfig: defaultCommonConfig(),
	}

	decodeConf := &mapstructure.DecoderConfig{
		DecodeHook:       ParseDurationFunc(),
		Result:           &config,
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decodeConf)
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(raw); err != nil {
		return nil, fmt.Errorf("error decoding config: %s", err)
	}

	if config.PrivateKey == "" && config.RootCert != "" {
		return nil, fmt.Errorf("must provide a private key when providing a root cert")
	}

	if err := config.CommonCAProviderConfig.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// ParseDurationFunc is a mapstructure hook for decoding a string or
// []uint8 into a time.Duration value.
func ParseDurationFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		var v time.Duration
		if t != reflect.TypeOf(v) {
			return data, nil
		}

		switch {
		case f.Kind() == reflect.String:
			if dur, err := time.ParseDuration(data.(string)); err != nil {
				return nil, err
			} else {
				v = dur
			}
			return v, nil
		case f == reflect.SliceOf(reflect.TypeOf(uint8(0))):
			s := Uint8ToString(data.([]uint8))
			if dur, err := time.ParseDuration(s); err != nil {
				return nil, err
			} else {
				v = dur
			}
			return v, nil
		default:
			return data, nil
		}
	}
}

func Uint8ToString(bs []uint8) string {
	b := make([]byte, len(bs))
	for i, v := range bs {
		b[i] = byte(v)
	}
	return string(b)
}

func defaultCommonConfig() structs.CommonCAProviderConfig {
	return structs.CommonCAProviderConfig{
		LeafCertTTL: 3 * 24 * time.Hour,
	}
}
