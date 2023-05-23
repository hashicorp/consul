package resourcetest

import (
	"context"
	"math/rand"
	"time"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	pbresource.ResourceServiceClient

	timeout time.Duration
	wait    time.Duration
}

func NewClient(client pbresource.ResourceServiceClient) *Client {
	return &Client{
		ResourceServiceClient: client,
		timeout:               7 * time.Second,
		wait:                  25 * time.Millisecond,
	}
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
	// Randomize the order of insertion. Generally insertion order shouldn't matter as the
	// controllers should eventually converge on the desired state. The exception to this
	// is that you cannot insert resources with owner refs before the resource they are
	// owned by or insert a resource into a non-default tenant before that tenant exists.
	rand.Shuffle(len(resources), func(i, j int) {
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
			_, err := client.Write(context.Background(), &pbresource.WriteRequest{
				Resource: res,
			})
			require.NoError(t, err)

			// track the number o
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

func (client *Client) RequireResourceNotFound(t T, id *pbresource.ID) {
	t.Helper()

	rsp, err := client.Read(context.Background(), &pbresource.ReadRequest{Id: id})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Nil(t, rsp)
}

func (client *Client) RequireResourceExists(t T, id *pbresource.ID) *pbresource.Resource {
	t.Helper()

	rsp, err := client.Read(context.Background(), &pbresource.ReadRequest{Id: id})
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

func (client *Client) MustDelete(t T, id *pbresource.ID) {
	_, err := client.Delete(context.Background(), &pbresource.DeleteRequest{Id: id})
	if status.Code(err) == codes.NotFound {
		return
	}

	require.NoError(t, err)
}
