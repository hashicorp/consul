package ca

import (
	"fmt"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/decode"
)

func ParseConsulCAConfig(raw map[string]interface{}) (*structs.ConsulCAProviderConfig, error) {
	config := structs.ConsulCAProviderConfig{
		CommonCAProviderConfig: defaultCommonConfig(),
	}
	decodeConf := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			structs.ParseDurationFunc(),
			decode.HookTranslateKeys,
		),
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

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

func defaultCommonConfig() structs.CommonCAProviderConfig {
	return structs.CommonCAProviderConfig{
		LeafCertTTL:         3 * 24 * time.Hour,
		IntermediateCertTTL: 24 * 365 * time.Hour,
		PrivateKeyType:      connect.DefaultPrivateKeyType,
		PrivateKeyBits:      connect.DefaultPrivateKeyBits,
		RootCertTTL:         10 * 24 * 365 * time.Hour,
	}
}
