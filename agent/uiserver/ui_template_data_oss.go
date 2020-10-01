// +build !consulent

package uiserver

import "github.com/hashicorp/consul/agent/config"

func uiTemplateDataFromConfigEnterprise(_ *config.RuntimeConfig, _ map[string]interface{}, _ map[string]interface{}) error {
	return nil
}
