package autoconf

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/hashicorp/consul/proto/pbautoconf"
)

const (
	// autoConfigFileName is the name of the file that the agent auto-config settings are
	// stored in within the data directory
	autoConfigFileName = "auto-config.json"
)

var (
	pbMarshaler = &jsonpb.Marshaler{
		OrigName:     false,
		EnumsAsInts:  false,
		Indent:       "   ",
		EmitDefaults: true,
	}

	pbUnmarshaler = &jsonpb.Unmarshaler{
		AllowUnknownFields: false,
	}
)

func (ac *AutoConfig) readPersistedAutoConfig() (*pbautoconf.AutoConfigResponse, error) {
	if ac.config.DataDir == "" {
		// no data directory means we don't have anything to potentially load
		return nil, nil
	}

	path := filepath.Join(ac.config.DataDir, autoConfigFileName)
	ac.logger.Debug("attempting to restore any persisted configuration", "path", path)

	content, err := ioutil.ReadFile(path)
	if err == nil {
		rdr := strings.NewReader(string(content))

		var resp pbautoconf.AutoConfigResponse
		if err := pbUnmarshaler.Unmarshal(rdr, &resp); err != nil {
			return nil, fmt.Errorf("failed to decode persisted auto-config data: %w", err)
		}

		ac.logger.Info("read persisted configuration", "path", path)
		return &resp, nil
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load %s: %w", path, err)
	}

	// ignore non-existence errors as that is an indicator that we haven't
	// performed the auto configuration before
	return nil, nil
}

func (ac *AutoConfig) persistAutoConfig(resp *pbautoconf.AutoConfigResponse) error {
	// now that we know the configuration is generally fine including TLS certs go ahead and persist it to disk.
	if ac.config.DataDir == "" {
		ac.logger.Debug("not persisting auto-config settings because there is no data directory")
		return nil
	}

	serialized, err := pbMarshaler.MarshalToString(resp)
	if err != nil {
		return fmt.Errorf("failed to encode auto-config response as JSON: %w", err)
	}

	path := filepath.Join(ac.config.DataDir, autoConfigFileName)

	err = ioutil.WriteFile(path, []byte(serialized), 0660)
	if err != nil {
		return fmt.Errorf("failed to write auto-config configurations: %w", err)
	}

	ac.logger.Debug("auto-config settings were persisted to disk")

	return nil
}
