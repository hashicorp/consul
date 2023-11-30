// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resourcetest

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

type ClientOption func(*Client)

func WithRNGSeed(seed int64) ClientOption {
	return func(c *Client) {
		c.rng = rand.New(rand.NewSource(seed))
	}
}

func WithRequestDelay(minMilliseconds int, maxMilliseconds int) ClientOption {
	return func(c *Client) {

		min := minMilliseconds
		max := maxMilliseconds
		if max < min {
			min = maxMilliseconds
			max = minMilliseconds
		}
		c.requestDelayMin = min
		c.requestDelayMax = max
	}
}

func WithACLToken(token string) ClientOption {
	return func(c *Client) {
		c.token = token
	}
}

type Client struct {
	pbresource.ResourceServiceClient

	timeout time.Duration
	wait    time.Duration
	token   string

	rng *rand.Rand

	requestDelayMin int
	requestDelayMax int
}

func NewClient(client pbresource.ResourceServiceClient, opts ...ClientOption) *Client {
	c := &Client{
		ResourceServiceClient: client,
		timeout:               7 * time.Second,
		wait:                  25 * time.Millisecond,
		rng:                   rand.New(rand.NewSource(time.Now().UnixNano())),
		// arbitrary write delays are opt-in only
		requestDelayMin: 0,
		requestDelayMax: 0,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func NewClientWithACLToken(client pbresource.ResourceServiceClient, token string) *Client {
	return NewClient(client, WithACLToken(token))
}

func (client *Client) SetRetryerConfig(timeout time.Duration, wait time.Duration) {
	client.timeout = timeout
	client.wait = wait
}

func (client *Client) retry(t T, fn func(r *retry.R)) {
	t.Helper()
	retryer := &retry.Timer{Timeout: client.timeout, Wait: client.wait}
	retry.RunWith(retryer, t, fn)
}

func (client *Client) PublishResources(t T, resources []*pbresource.Resource) {
	ctx := client.Context(t)

	// Randomize the order of insertion. Generally insertion order shouldn't matter as the
	// controllers should eventually converge on the desired state. The exception to this
	// is that you cannot insert resources with owner refs before the resource they are
	// owned by or insert a resource into a non-default tenant before that tenant exists.
	client.rng.Shuffle(len(resources), func(i, j int) {
		temp := resources[i]
		resources[i] = resources[j]
		resources[j] = temp
	})

	// This slice will be used to track the resources actually published each round. When
	// a resource with an owner ID is encountered we will not attempt to write but defer it
	// to the next round of publishing
	var written []*pbresource.ID

	for len(resources) > 0 {
		var left []*pbresource.Resource
		published := 0
		for _, res := range resources {

			// check that any owner references would be satisfied
			if res.Owner != nil {
				found := slices.ContainsFunc(written, func(id *pbresource.ID) bool {
					return resource.EqualID(res.Owner, id)
				})

				// the owner hasn't yet been published then we cannot publish this resource
				if !found {
					left = append(left, res)
					continue
				}
			}

			t.Logf("Writing resource %s with type %s", res.Id.Name, resource.ToGVK(res.Id.Type))
			rsp, err := client.Write(ctx, &pbresource.WriteRequest{
				Resource: res,
			})
			require.NoError(t, err)

			id := rsp.Resource.Id
			t.Cleanup(func() {
				client.CleanupDelete(t, id)
			})

			// track the number of resources published
			published += 1
			written = append(written, res.Id)
		}

		// the next round only has this subset of resources to attempt writing
		resources = left

		// if we didn't publish any resources this round then nothing would
		// enable us to do so by iterating again so we break to prevent infinite
		// loooping.
		if published == 0 {
			break
		}
	}

	require.Empty(t, resources, "Could not publish all resources - some resources have invalid owner references")
}

func (client *Client) Write(ctx context.Context, in *pbresource.WriteRequest, opts ...grpc.CallOption) (*pbresource.WriteResponse, error) {
	client.delayRequest()
	return client.ResourceServiceClient.Write(ctx, in, opts...)
}

func (client *Client) Context(t T) context.Context {
	ctx := testutil.TestContext(t)

	if client.token != "" {
		md := metadata.New(map[string]string{
			"x-consul-token": client.token,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	return ctx
}

func (client *Client) RequireResourceNotFound(t T, id *pbresource.ID) {
	t.Helper()

	rsp, err := client.Read(client.Context(t), &pbresource.ReadRequest{Id: id})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Nil(t, rsp)
}

func (client *Client) RequireResourceExists(t T, id *pbresource.ID) *pbresource.Resource {
	t.Helper()

	rsp, err := client.Read(client.Context(t), &pbresource.ReadRequest{Id: id})
	require.NoError(t, err, "error reading %s with type %s", id.Name, resource.ToGVK(id.Type))
	require.NotNil(t, rsp)
	return rsp.Resource
}

func (client *Client) RequireVersionUnchanged(t T, id *pbresource.ID, version string) *pbresource.Resource {
	t.Helper()

	res := client.RequireResourceExists(t, id)
	RequireVersionUnchanged(t, res, version)
	return res
}

func (client *Client) RequireVersionChanged(t T, id *pbresource.ID, version string) *pbresource.Resource {
	t.Helper()

	res := client.RequireResourceExists(t, id)
	RequireVersionChanged(t, res, version)
	return res
}

func (client *Client) RequireStatusCondition(t T, id *pbresource.ID, statusKey string, condition *pbresource.Condition) *pbresource.Resource {
	t.Helper()

	res := client.RequireResourceExists(t, id)
	RequireStatusCondition(t, res, statusKey, condition)
	return res
}

func (client *Client) RequireStatusConditionForCurrentGen(t T, id *pbresource.ID, statusKey string, condition *pbresource.Condition) *pbresource.Resource {
	t.Helper()

	res := client.RequireResourceExists(t, id)
	RequireStatusConditionForCurrentGen(t, res, statusKey, condition)
	return res
}

func (client *Client) RequireStatusConditionsForCurrentGen(t T, id *pbresource.ID, statusKey string, conditions []*pbresource.Condition) *pbresource.Resource {
	t.Helper()

	res := client.RequireResourceExists(t, id)
	for _, condition := range conditions {
		RequireStatusConditionForCurrentGen(t, res, statusKey, condition)
	}
	return res
}

func (client *Client) RequireResourceMeta(t T, id *pbresource.ID, key string, value string) *pbresource.Resource {
	t.Helper()

	res := client.RequireResourceExists(t, id)
	RequireResourceMeta(t, res, key, value)
	return res
}

func (client *Client) RequireReconciledCurrentGen(t T, id *pbresource.ID, statusKey string) *pbresource.Resource {
	t.Helper()

	res := client.RequireResourceExists(t, id)
	RequireReconciledCurrentGen(t, res, statusKey)
	return res
}

func (client *Client) WaitForReconciliation(t T, id *pbresource.ID, statusKey string) *pbresource.Resource {
	t.Helper()

	var res *pbresource.Resource

	client.retry(t, func(r *retry.R) {
		res = client.RequireReconciledCurrentGen(r, id, statusKey)
	})

	return res
}

func (client *Client) WaitForStatusCondition(t T, id *pbresource.ID, statusKey string, condition *pbresource.Condition) *pbresource.Resource {
	t.Helper()

	var res *pbresource.Resource
	client.retry(t, func(r *retry.R) {
		res = client.RequireStatusConditionForCurrentGen(r, id, statusKey, condition)
	})

	return res
}

func (client *Client) WaitForStatusConditionAnyGen(t T, id *pbresource.ID, statusKey string, condition *pbresource.Condition) *pbresource.Resource {
	t.Helper()

	var res *pbresource.Resource
	client.retry(t, func(r *retry.R) {
		res = client.RequireStatusCondition(r, id, statusKey, condition)
	})

	return res
}

func (client *Client) WaitForStatusConditions(t T, id *pbresource.ID, statusKey string, conditions ...*pbresource.Condition) *pbresource.Resource {
	t.Helper()

	var res *pbresource.Resource
	client.retry(t, func(r *retry.R) {
		res = client.RequireStatusConditionsForCurrentGen(r, id, statusKey, conditions)
	})

	return res
}

func (client *Client) WaitForNewVersion(t T, id *pbresource.ID, version string) *pbresource.Resource {
	t.Helper()

	var res *pbresource.Resource
	client.retry(t, func(r *retry.R) {
		res = client.RequireVersionChanged(r, id, version)
	})
	return res
}

func (client *Client) WaitForResourceState(t T, id *pbresource.ID, verify func(T, *pbresource.Resource)) *pbresource.Resource {
	t.Helper()

	var res *pbresource.Resource
	client.retry(t, func(r *retry.R) {
		res = client.RequireResourceExists(r, id)
		verify(r, res)
	})

	return res
}

func (client *Client) WaitForResourceExists(t T, id *pbresource.ID) *pbresource.Resource {
	t.Helper()

	var res *pbresource.Resource
	client.retry(t, func(r *retry.R) {
		res = client.RequireResourceExists(r, id)
	})

	return res
}

func (client *Client) WaitForDeletion(t T, id *pbresource.ID) {
	t.Helper()

	client.retry(t, func(r *retry.R) {
		client.RequireResourceNotFound(r, id)
	})
}

// ResolveResourceID will read the specified resource and returns its full ID.
// This is mainly useful to get the ID with the Uid filled out.
func (client *Client) ResolveResourceID(t T, id *pbresource.ID) *pbresource.ID {
	t.Helper()

	return client.RequireResourceExists(t, id).Id
}

// MustDelete will delete a resource by its id, retrying if necessary and fail the test
// if it cannot delete it within the timeout. The clients request delay settings are
// taken into account with this operation.
func (client *Client) MustDelete(t T, id *pbresource.ID) {
	t.Helper()
	client.retryDelete(t, id, true)
}

// CleanupDelete will perform the same operations as MustDelete to ensure the resource is
// deleted. The clients request delay settings are ignored for this operation and it is
// assumed this will only be called in the context of test Cleanup routines where we
// are no longer testing that a controller eventually converges on some values in response
// to the delete.
func (client *Client) CleanupDelete(t T, id *pbresource.ID) {
	t.Helper()
	client.retryDelete(t, id, false)
}

func (client *Client) retryDelete(t T, id *pbresource.ID, shouldDelay bool) {
	t.Helper()
	ctx := client.Context(t)

	client.retry(t, func(r *retry.R) {
		if shouldDelay {
			client.delayRequest()
		}
		_, err := client.Delete(ctx, &pbresource.DeleteRequest{Id: id})
		if status.Code(err) == codes.NotFound {
			return
		}

		// codes.Aborted indicates a CAS failure and that the delete request should
		// be retried. Anything else should be considered an unrecoverable error.
		if err != nil && status.Code(err) != codes.Aborted {
			r.Stop(fmt.Errorf("failed to delete the resource: %w", err))
			return
		}

		require.NoError(r, err)
	})
}

func (client *Client) delayRequest() {
	if client.requestDelayMin == 0 && client.requestDelayMax == 0 {
		return
	}

	var delay time.Duration
	if client.requestDelayMin == client.requestDelayMax {
		delay = time.Duration(client.requestDelayMin) * time.Millisecond
	} else {
		delay = time.Duration(client.rng.Intn(client.requestDelayMax-client.requestDelayMin)+client.requestDelayMin) * time.Millisecond
	}
	time.Sleep(delay)
}

type CLIOptions struct {
	minRequestDelay int
	maxRequestDelay int
	seed            int64
}

type CLIOptionT interface {
	Helper()
	Logf(string, ...any)
}

func (o *CLIOptions) ClientOptions(t CLIOptionT) []ClientOption {
	t.Helper()
	t.Logf("Using %d for the random number generator seed. Pass -rng-seed=<value> to overwrite the time based seed", o.seed)
	t.Logf("Using random request delays between %dms and %dms. Use -min-request-delay=<value> or -max-request-delay=<value> to override the defaults", o.minRequestDelay, o.maxRequestDelay)

	return []ClientOption{
		WithRNGSeed(o.seed),
		WithRequestDelay(o.minRequestDelay, o.maxRequestDelay),
	}
}

func ConfigureTestCLIFlags() *CLIOptions {
	opts := &CLIOptions{
		minRequestDelay: 0,
		maxRequestDelay: 0,
		seed:            time.Now().UnixNano(),
	}

	flag.Int64Var(&opts.seed, "rng-seed", opts.seed, "Seed to use for pseudo-random-number-generators")
	flag.IntVar(&opts.minRequestDelay, "min-request-delay", 10, "Minimum delay before performing a resource write (milliseconds: default=10)")
	flag.IntVar(&opts.maxRequestDelay, "max-request-delay", 50, "Maximum delay before performing a resource write (milliseconds: default=50)")

	return opts
}
