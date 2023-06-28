package tfgen

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/hashicorp/consul/testingconsul"
	"golang.org/x/crypto/blake2b"
)

const proxyInternalPort = 80

func (g *Generator) writeNginxConfig(net *testingconsul.Network) (bool, string, error) {
	rootdir := filepath.Join(g.workdir, "terraform", "nginx-config-"+net.Name)
	if err := os.MkdirAll(rootdir, 0755); err != nil {
		return false, "", err
	}

	configFile := filepath.Join(rootdir, "nginx.conf")

	body := fmt.Sprintf(`
server {
    listen       %d;

    location / {
        resolver 8.8.8.8;
        ##############
        # Relevant config knobs are here: https://nginx.org/en/docs/http/ngx_http_proxy_module.html
        ##############
        proxy_pass http://$http_host$uri$is_args$args;
        proxy_cache off;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_connect_timeout 5s;
        proxy_read_timeout 5s;
        proxy_send_timeout 5s;
        proxy_request_buffering off;
        proxy_buffering off;
    }
}
`, proxyInternalPort)

	_, err := UpdateFileIfDifferent(
		g.logger,
		[]byte(body),
		configFile,
		0644,
	)
	if err != nil {
		return false, "", fmt.Errorf("error writing %q: %w", configFile, err)
	}

	hash, err := hashFile(configFile)
	if err != nil {
		return false, "", fmt.Errorf("error hashing %q: %w", configFile, err)
	}

	return true, hash, err
}

func (g *Generator) getForwardProxyContainer(
	net *testingconsul.Network,
	ipAddress string,
	hash string,
) Resource {
	env := []string{"HASH_FILE_VALUE=" + hash}
	proxy := struct {
		Name              string
		DockerNetworkName string
		InternalPort      int
		IPAddress         string
		Env               []string
	}{
		Name:              net.Name,
		DockerNetworkName: net.DockerName,
		InternalPort:      proxyInternalPort,
		IPAddress:         ipAddress,
		Env:               env,
	}

	return Eval(tfForwardProxyT, &proxy)
}

var tfForwardProxyT = template.Must(template.ParseFS(content, "templates/container-proxy.tf.tmpl"))

func hashFile(path string) (string, error) {
	hash, err := blake2b.New256(nil)
	if err != nil {
		return "", err
	}

	if err := addFileToHash(path, hash); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func addFileToHash(path string, w io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}
