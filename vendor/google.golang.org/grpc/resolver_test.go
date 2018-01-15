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
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/grpc/test/leakcheck"
)

func TestResolverServiceConfigBeforeAddressNotPanic(t *testing.T) {
	defer leakcheck.Check(t)
	r, rcleanup := manual.GenerateAndRegisterManualResolver()
	defer rcleanup()

	cc, err := Dial(r.Scheme()+":///test.server", WithInsecure())
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer cc.Close()

	// SwitchBalancer before NewAddress. There was no balancer created, this
	// makes sure we don't call close on nil balancerWrapper.
	r.NewServiceConfig(`{"loadBalancingPolicy": "round_robin"}`) // This should not panic.

	time.Sleep(time.Second) // Sleep to make sure the service config is handled by ClientConn.
}

func TestResolverEmptyUpdateNotPanic(t *testing.T) {
	defer leakcheck.Check(t)
	r, rcleanup := manual.GenerateAndRegisterManualResolver()
	defer rcleanup()

	cc, err := Dial(r.Scheme()+":///test.server", WithInsecure())
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer cc.Close()

	// This make sure we don't create addrConn with empty address list.
	r.NewAddress([]resolver.Address{}) // This should not panic.

	time.Sleep(time.Second) // Sleep to make sure the service config is handled by ClientConn.
}

var (
	errTestResolverFailBuild = fmt.Errorf("test resolver build error")
)

type testResolverFailBuilder struct {
	buildOpt resolver.BuildOption
}

func (r *testResolverFailBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOption) (resolver.Resolver, error) {
	r.buildOpt = opts
	return nil, errTestResolverFailBuild
}
func (r *testResolverFailBuilder) Scheme() string {
	return "testResolverFailBuilderScheme"
}

// Tests that options in WithResolverUserOptions are passed to resolver.Build().
func TestResolverUserOptions(t *testing.T) {
	r := &testResolverFailBuilder{}

	userOpt := "testUserOpt"
	_, err := Dial("scheme:///test.server", WithInsecure(),
		withResolverBuilder(r),
		WithResolverUserOptions(userOpt),
	)
	if err == nil || !strings.Contains(err.Error(), errTestResolverFailBuild.Error()) {
		t.Fatalf("Dial with testResolverFailBuilder returns err: %v, want: %v", err, errTestResolverFailBuild)
	}

	if r.buildOpt.UserOptions != userOpt {
		t.Fatalf("buildOpt.UserOptions = %T %+v, want %v", r.buildOpt.UserOptions, r.buildOpt.UserOptions, userOpt)
	}
}
