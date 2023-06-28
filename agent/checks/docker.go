// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package checks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/armon/circbuf"
	"github.com/docker/go-connections/sockets"
)

// DockerClient is a simplified client for the Docker Engine API
// to execute the health checks and avoid significant dependencies.
// It also consumes all data returned from the Docker API through
// a ring buffer with a fixed limit to avoid excessive resource
// consumption.
type DockerClient struct {
	host     string
	scheme   string
	proto    string
	addr     string
	basepath string
	maxbuf   int64
	client   *http.Client
}

func NewDockerClient(host string, maxbuf int64) (*DockerClient, error) {
	if host == "" {
		host = DefaultDockerHost
	}

	proto, addr, basepath, err := ParseHost(host)
	if err != nil {
		return nil, err
	}

	transport := new(http.Transport)
	sockets.ConfigureTransport(transport, proto, addr)
	client := &http.Client{Transport: transport}

	return &DockerClient{
		host:     host,
		scheme:   "http",
		proto:    proto,
		addr:     addr,
		basepath: basepath,
		maxbuf:   maxbuf,
		client:   client,
	}, nil
}

func (c *DockerClient) Close() error {
	if t, ok := c.client.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
	return nil
}

func (c *DockerClient) Host() string {
	return c.host
}

// ParseHost verifies that the given host strings is valid.
// copied from github.com/docker/docker/client.go
func ParseHost(host string) (string, string, string, error) {
	protoAddrParts := strings.SplitN(host, "://", 2)
	if len(protoAddrParts) == 1 {
		return "", "", "", fmt.Errorf("unable to parse docker host `%s`", host)
	}

	var basePath string
	proto, addr := protoAddrParts[0], protoAddrParts[1]
	if proto == "tcp" {
		parsed, err := url.Parse("tcp://" + addr)
		if err != nil {
			return "", "", "", err
		}
		addr = parsed.Host
		basePath = parsed.Path
	}
	return proto, addr, basePath, nil
}

func (c *DockerClient) call(method, uri string, v interface{}) (*circbuf.Buffer, int, error) {
	req, err := http.NewRequest(method, uri, nil)
	if err != nil {
		return nil, 0, err
	}

	if c.proto == "unix" || c.proto == "npipe" {
		// For local communications, it doesn't matter what the host is. We just
		// need a valid and meaningful host name. (See #189)
		req.Host = "docker"
	}

	req.URL.Host = c.addr
	req.URL.Scheme = c.scheme

	if v != nil {
		var b bytes.Buffer
		if err := json.NewEncoder(&b).Encode(v); err != nil {
			return nil, 0, err
		}
		req.Body = io.NopCloser(&b)
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	b, err := circbuf.NewBuffer(c.maxbuf)
	if err != nil {
		return nil, 0, err
	}
	_, err = io.Copy(b, resp.Body)
	return b, resp.StatusCode, err
}

func (c *DockerClient) CreateExec(containerID string, cmd []string) (string, error) {
	data := struct {
		AttachStdin  bool
		AttachStdout bool
		AttachStderr bool
		Tty          bool
		Cmd          []string
	}{
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          cmd,
	}

	uri := fmt.Sprintf("/containers/%s/exec", url.QueryEscape(containerID))
	b, code, err := c.call("POST", uri, data)
	switch {
	case err != nil:
		return "", fmt.Errorf("create exec failed for container %s: %v", containerID, err)
	case code == 201:
		var resp struct{ Id string }
		if err = json.NewDecoder(bytes.NewReader(b.Bytes())).Decode(&resp); err != nil {
			return "", fmt.Errorf("create exec response for container %s cannot be parsed: %s", containerID, err)
		}
		return resp.Id, nil
	case code == 404:
		return "", fmt.Errorf("create exec failed for unknown container %s", containerID)
	case code == 409:
		return "", fmt.Errorf("create exec failed since container %s is paused or stopped", containerID)
	default:
		return "", fmt.Errorf("create exec failed for container %s with status %d: %s", containerID, code, b)
	}
}

func (c *DockerClient) StartExec(containerID, execID string) (*circbuf.Buffer, error) {
	data := struct{ Detach, Tty bool }{Detach: false, Tty: true}
	uri := fmt.Sprintf("/exec/%s/start", execID)
	b, code, err := c.call("POST", uri, data)
	switch {
	// todo(fs): https://github.com/hashicorp/consul/pull/3621
	// todo(fs): for some reason the docker agent closes the connection during the
	// todo(fs): io.Copy call in c.call which causes a "connection reset by peer" error
	// todo(fs): even though both body and status code have been received. My current is
	// todo(fs): that the docker agent closes this prematurely but I don't understand why.
	// todo(fs): the code below ignores this error.
	case err != nil && !strings.Contains(err.Error(), "connection reset by peer"):
		return nil, fmt.Errorf("start exec failed for container %s: %v", containerID, err)
	case code == 200:
		return b, nil
	case code == 404:
		return nil, fmt.Errorf("start exec failed for container %s: invalid exec id %s", containerID, execID)
	case code == 409:
		return nil, fmt.Errorf("start exec failed since container %s is paused or stopped", containerID)
	default:
		return nil, fmt.Errorf("start exec failed for container %s with status %d: body: %s err: %v", containerID, code, b, err)
	}
}

func (c *DockerClient) InspectExec(containerID, execID string) (int, error) {
	uri := fmt.Sprintf("/exec/%s/json", execID)
	b, code, err := c.call("GET", uri, nil)
	switch {
	case err != nil:
		return 0, fmt.Errorf("inspect exec failed for container %s: %s", containerID, err)
	case code == 200:
		var resp struct{ ExitCode int }
		if err := json.NewDecoder(bytes.NewReader(b.Bytes())).Decode(&resp); err != nil {
			return 0, fmt.Errorf("inspect exec response for container %s cannot be parsed: %v", containerID, err)
		}
		return resp.ExitCode, nil
	case code == 404:
		return 0, fmt.Errorf("inspect exec failed for container %s: invalid exec id %s", containerID, execID)
	default:
		return 0, fmt.Errorf("inspect exec failed for container %s with status %d: %s", containerID, code, b)
	}
}
