package cloudcfg

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/hashicorp/consul/agent/config"
)

type CloudConfig struct {
	ResourceID   string `json:"resource_id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

func loadCloudConfig(dir string) (*CloudConfig, error) {
	p := filepath.Join(dir, "cloud.json")
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	cloudConfig := CloudConfig{}
	if err := json.Unmarshal(data, &cloudConfig); err != nil {
		return nil, err
	}
	return &cloudConfig, nil
}

func CloudConfigLoader(baseLoader func(source config.Source) (config.LoadResult, error)) func(source config.Source) (config.LoadResult, error) {
	return func(source config.Source) (config.LoadResult, error) {
		res, err := baseLoader(source)
		if err != nil {
			return res, err
		}

		if res.RuntimeConfig.DevMode ||
			res.RuntimeConfig.Cloud.ResourceID != "" ||
			res.RuntimeConfig.Cloud.ClientID != "" ||
			res.RuntimeConfig.Cloud.ClientSecret != "" {
			return res, nil
		}

		// load from the cloud.json file
		dir := filepath.Join(res.RuntimeConfig.DataDir, "hcp-config")
		cloudCfg, err := loadCloudConfig(dir)
		if err != nil {
			return res, err
		}

		if cloudCfg == nil {
			return res, nil
		}

		res.RuntimeConfig.Bootstrap = true
		res.RuntimeConfig.Cloud.ResourceID = cloudCfg.ResourceID
		res.RuntimeConfig.Cloud.ClientID = cloudCfg.ClientID
		res.RuntimeConfig.Cloud.ClientSecret = cloudCfg.ClientSecret
		return res, err
	}
}
