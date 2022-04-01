package vaultcontainer

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/docker/pkg/ioutils"

	"github.com/hashicorp/consul/integration/ca/libs/utils"

	"github.com/testcontainers/testcontainers-go"

	"github.com/hashicorp/vault/api"

	"github.com/testcontainers/testcontainers-go/wait"
)

type vaultNode struct {
	ctx    context.Context
	Client *api.Client
}

const bootLogLine = "New leader elected"

func NewNodeWitConfig(ctx context.Context, config string) (*vaultNode, error) {
	name := utils.RandName("vault-")
	ctx = context.WithValue(ctx, "name", name)
	tmpDir, err := ioutils.TempDir("/tmp", name)
	if err != nil {
		return nil, err
	}
	err = os.Mkdir(tmpDir+"/config", 0777)
	if err != nil {
		return nil, err
	}
	configFile := tmpDir + "/config/config.hcl"
	err = os.WriteFile(configFile, []byte(config), 0644)
	if err != nil {
		return nil, err
	}
	req := testcontainers.ContainerRequest{
		Image:        "vault",
		ExposedPorts: []string{"8500/tcp"},
		WaitingFor:   wait.ForLog(bootLogLine),
		AutoRemove:   false,
		Name:         name,
		BindMounts:   map[string]string{"/vault/config/config.hcl": configFile},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	ip, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, "8500")
	if err != nil {
		return nil, err
	}

	uri := fmt.Sprintf("http://%s:%s", ip, mappedPort.Port())
	c := new(vaultNode)
	apiConfig := api.DefaultConfig()
	apiConfig.Address = uri
	c.Client, err = api.NewClient(apiConfig)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func NewNode() (*vaultNode, error) {
	return NewNodeWitConfig(context.Background(), "")
}
