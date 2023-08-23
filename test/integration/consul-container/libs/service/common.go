package service

import (
	"archive/tar"
	"bytes"
	_ "embed"
	"os"

	"github.com/testcontainers/testcontainers-go"
)

const (
	envoyEnvKey   = "ENVOY_VERSION"
	envoyLogLevel = "debug"
	envoyVersion  = "1.23.1"

	hashicorpDockerProxy = "docker.mirror.hashicorp.services"
)

//go:embed assets/Dockerfile-consul-envoy
var consulEnvoyDockerfile string

// getDevContainerDockerfile returns the necessary context to build a combined consul and
// envoy image for running "consul connect envoy ..."
func getDevContainerDockerfile() (testcontainers.FromDockerfile, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	dockerfileBytes := []byte(consulEnvoyDockerfile)

	hdr := &tar.Header{
		Name: "Dockerfile",
		Mode: 0600,
		Size: int64(len(dockerfileBytes)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return testcontainers.FromDockerfile{}, err
	}

	if _, err := tw.Write(dockerfileBytes); err != nil {
		return testcontainers.FromDockerfile{}, err
	}

	if err := tw.Close(); err != nil {
		return testcontainers.FromDockerfile{}, err
	}
	reader := bytes.NewReader(buf.Bytes())
	fromDockerfile := testcontainers.FromDockerfile{
		ContextArchive: reader,
	}

	return fromDockerfile, nil
}

func getEnvoyVersion() string {
	if version, ok := os.LookupEnv(envoyEnvKey); ok && version != "" {
		return version
	}
	return envoyVersion
}
