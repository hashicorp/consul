package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/armon/circbuf"
)

// DockerClient is a simplified client for the Docker Engine API
// to execute the health checks and avoid significant dependencies.
// It also consumes all data returned from the Docker API through
// a ring buffer with a fixed limit to avoid excessive resource
// consumption.
type DockerClient struct {
	network string
	addr    string
	baseurl string
	maxbuf  int64
	client  *http.Client
}

func NewDockerClient(host string, maxbuf int64) (*DockerClient, error) {
	if host == "" {
		host = DefaultDockerHost
	}
	p := strings.SplitN(host, "://", 2)
	if len(p) != 2 {
		return nil, fmt.Errorf("invalid docker host: %s", host)
	}
	network, addr := p[0], p[1]
	basepath := "http://" + addr
	if network == "unix" {
		basepath = "http://unix"
	}
	client := &http.Client{}
	if network == "unix" {
		client.Transport = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		}
	}
	return &DockerClient{network, addr, basepath, maxbuf, client}, nil
}

func (c *DockerClient) call(method, uri string, v interface{}) (*circbuf.Buffer, int, error) {
	urlstr := c.baseurl + uri
	req, err := http.NewRequest(method, urlstr, nil)
	if err != nil {
		return nil, 0, err
	}

	if v != nil {
		var b bytes.Buffer
		if err := json.NewEncoder(&b).Encode(v); err != nil {
			return nil, 0, err
		}
		req.Body = ioutil.NopCloser(&b)
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
		return "", fmt.Errorf("create exec failed for container %s: %s", containerID, err)
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
	case err != nil:
		return nil, fmt.Errorf("start exec failed for container %s: %s", containerID, err)
	case code == 200:
		return b, nil
	case code == 404:
		return nil, fmt.Errorf("start exec failed for container %s: invalid exec id %s", containerID, execID)
	case code == 409:
		return nil, fmt.Errorf("start exec failed since container %s is paused or stopped", containerID)
	default:
		return nil, fmt.Errorf("start exec failed for container %s with status %d: %s", containerID, code, b)
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
			return 0, fmt.Errorf("inspect exec response for container %s cannot be parsed: %s", containerID, err)
		}
		return resp.ExitCode, nil
	case code == 404:
		return 0, fmt.Errorf("inspect exec failed for container %s: invalid exec id %s", containerID, execID)
	default:
		return 0, fmt.Errorf("inspect exec failed for container %s with status %d: %s", containerID, code, b)
	}
}
