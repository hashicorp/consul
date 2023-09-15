package cluster

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

const (
	certVolumePrefix = "test-container-certs-"
	consulUID        = "100"
	consulGID        = "1000"
	consulUserArg    = consulUID + ":" + consulGID
)

func newSerfEncryptionKey() (string, error) {
	key := make([]byte, 32)
	n, err := rand.Reader.Read(key)
	if err != nil {
		return "", errors.Wrap(err, "error reading random data")
	}
	if n != 32 {
		return "", errors.Wrap(err, "couldn't read enough entropy. Generate more entropy!")
	}

	return base64.StdEncoding.EncodeToString(key), nil
}

func (c *BuildContext) createTLSCAFiles(t *testing.T) {
	id, err := uuid.GenerateUUID()
	require.NoError(t, err, "could not create cert volume UUID")

	c.certVolume = certVolumePrefix + id

	// TODO: cleanup anything with the prefix?

	// Create a volume to hold the data.
	err = utils.DockerExec([]string{"volume", "create", c.certVolume}, io.Discard)
	require.NoError(t, err, "could not create docker volume to hold cert data: %s", c.certVolume)
	t.Cleanup(func() {
		_ = utils.DockerExec([]string{"volume", "rm", c.certVolume}, io.Discard)
	})

	err = utils.DockerExec([]string{"run",
		"--rm",
		"-i",
		"--net=none",
		"-v", c.certVolume + ":/data",
		"busybox:latest",
		"sh", "-c",
		// Need this so the permissions stick; docker seems to treat unused volumes differently.
		`touch /data/VOLUME_PLACEHOLDER && chown -R ` + consulUserArg + ` /data`,
	}, io.Discard)
	require.NoError(t, err, "could not initialize docker volume for cert data: %s", c.certVolume)

	err = utils.DockerExec([]string{"run",
		"--rm",
		"-i",
		"--net=none",
		"-u", consulUserArg,
		"-v", c.certVolume + ":/data",
		"-w", "/data",
		"--entrypoint", "",
		c.DockerImage(),
		"consul", "tls", "ca", "create",
	}, io.Discard)
	require.NoError(t, err, "could not create TLS certificate authority in docker volume: %s", c.certVolume)

	var w bytes.Buffer
	err = utils.DockerExec([]string{"run",
		"--rm",
		"-i",
		"--net=none",
		"-u", consulUserArg,
		"-v", c.certVolume + ":/data",
		"-w", "/data",
		"--entrypoint", "",
		c.DockerImage(),
		"cat", filepath.Join("/data", ConsulCACertPEM),
	}, &w)
	require.NoError(t, err, "could not extract TLS CA certificate authority public key from docker volume: %s", c.certVolume)

	c.caCert = w.String()
}

func (c *BuildContext) createTLSCertFiles(t *testing.T, dc string) (keyFileName, certFileName string) {
	require.NotEmpty(t, "the CA has not been initialized yet")

	err := utils.DockerExec([]string{"run",
		"--rm",
		"-i",
		"--net=none",
		"-u", consulUserArg,
		"-v", c.certVolume + ":/data",
		"-w", "/data",
		"--entrypoint", "",
		c.DockerImage(),
		"consul", "tls", "cert", "create", "-server", "-dc", dc,
	}, io.Discard)
	require.NoError(t, err, "could not create TLS server certificate dc=%q in docker volume: %s", dc, c.certVolume)

	prefix := fmt.Sprintf("%s-server-%s", dc, "consul")
	certFileName = fmt.Sprintf("%s-%d.pem", prefix, c.tlsCertIndex)
	keyFileName = fmt.Sprintf("%s-%d-key.pem", prefix, c.tlsCertIndex)

	for _, fn := range []string{certFileName, keyFileName} {
		err = utils.DockerExec([]string{"run",
			"--rm",
			"-i",
			"--net=none",
			"-u", consulUserArg,
			"-v", c.certVolume + ":/data:ro",
			"-w", "/data",
			"--entrypoint", "",
			c.DockerImage(),
			"stat", filepath.Join("/data", fn),
		}, io.Discard)
		require.NoError(t, err, "Generated TLS cert file %q does not exist in volume", fn)
	}

	return keyFileName, certFileName
}
