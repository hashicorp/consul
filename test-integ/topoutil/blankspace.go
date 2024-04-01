// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topoutil

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rboyer/blankspace/blankpb"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GetBlankspaceNameViaHTTP calls a copy of blankspace once via HTTP and
// retrieves the self-identified name of the instance.
func GetBlankspaceNameViaHTTP(
	ctx context.Context,
	client *http.Client,
	serverAddr string,
	actualURL string,
) (string, error) {
	url := fmt.Sprintf("http://%s/fetch?url=%s", serverAddr,
		url.QueryEscape(actualURL),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code is not 200: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	var v struct {
		Name string
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return "", err
	}

	if _, useHTTP2 := client.Transport.(*http2.Transport); useHTTP2 {
		if res.ProtoMajor < 2 {
			return "", fmt.Errorf("should be using http > 1.x not %d", res.ProtoMajor)
		}
	}

	return v.Name, nil
}

// GetBlankspaceNameViaGRPC calls a copy of blankspace once via gRPC and
// retrieves the self-identified name of the instance.
func GetBlankspaceNameViaGRPC(ctx context.Context, serverAddr string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", err
	}
	defer conn.Close()

	client := blankpb.NewServerClient(conn)

	resp, err := client.Describe(ctx, &blankpb.DescribeRequest{})
	if err != nil {
		return "", fmt.Errorf("grpc error from Describe: %w", err)
	}

	return resp.GetName(), nil
}

// GetBlankspaceNameViaTCP calls a copy of blankspace once via tcp and
// retrieves the self-identified name of the instance.
func GetBlankspaceNameViaTCP(ctx context.Context, serverAddr string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	d := net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 250 * time.Millisecond,
	}

	conn, err := d.DialContext(ctx, "tcp", serverAddr)
	if err != nil {
		return "", fmt.Errorf("tcp error dialing: %w", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("describe\n")); err != nil {
		return "", fmt.Errorf("error sending tcp request: %w", err)
	}

	scan := bufio.NewScanner(conn)

	if !scan.Scan() {
		return "", fmt.Errorf("server did not reply")
	}

	name := strings.TrimSpace(scan.Text())

	return name, nil
}
