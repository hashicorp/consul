/*
 *
 * Copyright 2017 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package grpc

import (
	"fmt"
	"math"
	"testing"
	"time"

	"golang.org/x/net/context"
	_ "google.golang.org/grpc/grpclog/glogger"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/grpc/test/leakcheck"
)

func checkPickFirst(cc *ClientConn, servers []*server) error {
	var (
		req   = "port"
		reply string
		err   error
	)
	connected := false
	for i := 0; i < 1000; i++ {
		if err = Invoke(context.Background(), "/foo/bar", &req, &reply, cc); ErrorDesc(err) == servers[0].port {
			if connected {
				// connected is set to false if peer is not server[0]. So if
				// connected is true here, this is the second time we saw
				// server[0] in a row. Break because pickfirst is in effect.
				break
			}
			connected = true
		} else {
			connected = false
		}
		time.Sleep(time.Millisecond)
	}
	if !connected {
		return fmt.Errorf("pickfirst is not in effect after 1 second, EmptyCall() = _, %v, want _, %v", err, servers[0].port)
	}
	// The following RPCs should all succeed with the first server.
	for i := 0; i < 3; i++ {
		err = Invoke(context.Background(), "/foo/bar", &req, &reply, cc)
		if ErrorDesc(err) != servers[0].port {
			return fmt.Errorf("Index %d: want peer %v, got peer %v", i, servers[0].port, err)
		}
	}
	return nil
}

func checkRoundRobin(cc *ClientConn, servers []*server) error {
	var (
		req   = "port"
		reply string
		err   error
	)

	// Make sure connections to all servers are up.
	for i := 0; i < 2; i++ {
		// Do this check twice, otherwise the first RPC's transport may still be
		// picked by the closing pickfirst balancer, and the test becomes flaky.
		for _, s := range servers {
			var up bool
			for i := 0; i < 1000; i++ {
				if err = Invoke(context.Background(), "/foo/bar", &req, &reply, cc); ErrorDesc(err) == s.port {
					up = true
					break
				}
				time.Sleep(time.Millisecond)
			}
			if !up {
				return fmt.Errorf("server %v is not up within 1 second", s.port)
			}
		}
	}

	serverCount := len(servers)
	for i := 0; i < 3*serverCount; i++ {
		err = Invoke(context.Background(), "/foo/bar", &req, &reply, cc)
		if ErrorDesc(err) != servers[i%serverCount].port {
			return fmt.Errorf("Index %d: want peer %v, got peer %v", i, servers[i%serverCount].port, err)
		}
	}
	return nil
}

func TestSwitchBalancer(t *testing.T) {
	defer leakcheck.Check(t)
	r, rcleanup := manual.GenerateAndRegisterManualResolver()
	defer rcleanup()

	numServers := 2
	servers, _, scleanup := startServers(t, numServers, math.MaxInt32)
	defer scleanup()

	cc, err := Dial(r.Scheme()+":///test.server", WithInsecure(), WithCodec(testCodec{}))
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer cc.Close()
	r.NewAddress([]resolver.Address{{Addr: servers[0].addr}, {Addr: servers[1].addr}})
	// The default balancer is pickfirst.
	if err := checkPickFirst(cc, servers); err != nil {
		t.Fatalf("check pickfirst returned non-nil error: %v", err)
	}
	// Switch to roundrobin.
	cc.handleServiceConfig(`{"loadBalancingPolicy": "round_robin"}`)
	if err := checkRoundRobin(cc, servers); err != nil {
		t.Fatalf("check roundrobin returned non-nil error: %v", err)
	}
	// Switch to pickfirst.
	cc.handleServiceConfig(`{"loadBalancingPolicy": "pick_first"}`)
	if err := checkPickFirst(cc, servers); err != nil {
		t.Fatalf("check pickfirst returned non-nil error: %v", err)
	}
}
