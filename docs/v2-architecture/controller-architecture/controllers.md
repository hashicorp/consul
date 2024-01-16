# Controllers

This page describes how to write controllers in Consul's new controller architecture.

-> **Note**: This information is valid as of Consul 1.17 but some portions may change in future releases.

## Controller Basics

A controller consists of several parts: 

1. **The watched type** - This is the main type a controller is watching and reconciling.
2. **Additional watched types** - These are additional types a controller may care about in addition to the main watched type.
3. **Additional custom watches** - These are the watches for things that aren't resources in Consul. 
4. **Reconciler** - This is the instance that's responsible for reconciling requests whenever there's an event for the main watched type or for any of the watched types.

A basic controller setup could look like this:

```go
func barController() controller.Controller {
    return controller.NewController("bar", pbexample.BarType).
        WithReconciler(barReconciler{})
}
```

barReconciler needs to implement the `Reconcile` method of the `Reconciler` interface. 
It's important to note that the `Reconcile` method only gets the request with the `ID` of the main
watched resource and so it's up to the reconcile implementation to fetch the resource and any relevant information needed
to perform the reconciliation. The most basic reconciler could look as follows:

```go
type barReconciler struct {}

func (b *barReconciler) Reconcile(ctx context.Context, rt Runtime, req Request) error {
    ...
}
```

## Watching Additional Resources

Most of the time, controllers will need to watch more resources in addition to the main watched type. 
To set up an additional watch, the main thing we need to figure out is how to map additional watched resource to the main
watched resource. Controller-runtime allows us to implement a mapper function that can take the additional watched resource
as the input and produce reconcile `Requests` for our main watched type.

To figure out how to map the two resources together, we need to think about the relationship between the two resources.

There are several common relationship types between resources that are being used currently:
1. Name-alignment: this relationship means that resources are named the same and live in the same tenancy, but have different data. Examples: `Service` and `ServiceEndpoints`, `Workload` and `ProxyStateTemplate`.
2. Selector: this relationship happens when one resource selects another by name or name prefix. Examples: `Service` and `Workload`, `ProxyConfiguration` and `Workload`.
3. Owner: in this relationship, one resource is the owner of another resource. Examples: `Service` and `ServiceEndpoints`, `HealthStatus` and `Workload`.
4. Arbitrary reference: in this relationship, one resource may reference another by some sort of reference. This reference could be a single string in the resource data or a more composite reference containing name, tenancy, and type. Examples: `Workload` and `WorkloadIdentity`, `HTTPRoute` and `Service`.

Note that it's possible for the two watched resources to have more than one relationship type simultaneously. 
For example, `FailoverPolicy` type is name-aligned with a service to which it applies, however, it also contains
references to destination services, and for a controller that reconciles `FailoverPolicy` and watches `Service`
we need to account for both type 1 and type 4 relationship whenever we get an event for a `Service`. 

### Simple Mappers

Let's look at some simple mapping examples. 

#### Name-aligned resources
If our resources only have a name-aligned relationship, we can map them with a built-in function:

```go
func barController() controller.Controller {
    return controller.NewController("bar", pbexample.BarType).
        WithWatch(pbexample.FooType, controller.ReplaceType(pbexample.BarType)). 
        WithReconciler(barReconciler{})
}
```

Here, all we need to do is replace the type of the `Foo` resource whenever we get an event for it.

#### Owned resources

Let's say our `Foo` resource owns `Bar` resources, where any `Foo` resource can own multiple `Bar` resources. 
In this case, whenever we see a new event for `Foo`, all we need to do is get all `Bar` resources that `Foo` currently owns.
For this, we can also use a built-in function to set up our watch:

```go
func MapOwned(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
    resp, err := rt.Client.ListByOwner(ctx, &pbresource.ListByOwnerRequest{Owner: res.Id})
    if err != nil {
        return nil, err
    }

    var result []controller.Request
    for _, r := range resp.Resources {
        result = append(result, controller.Request{ID: r.Id})
    }

    return result, nil
}

func barController() controller.Controller {
    return controller.NewController("bar", pbexample.BarType).
        WithWatch(pbexample.FooType, MapOwned). 
        WithReconciler(barReconciler{})
}
```

### Advanced Mappers and Caches

For selector or arbitrary reference relationships, the mapping that we choose may need to be more advanced. 

#### Naive mapper implementation

Let's first consider what a naive mapping function could look like in this case. Let's say that the `Bar` resource
references `Foo` resource by name in the data. Now to watch and map `Foo` resources, we need to be able to find all relevant `Bar` resources
whenever we get an event for a `Foo` resource.

```go
func MapFoo(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
    resp, err := rt.Client.List(ctx, &pbresource.ListRequest{Type: pbexample.BarType, Tenancy: res.Id.Tenancy})
    if err != nil {
        return nil, err
    }

    var result []controller.Request
    for _, r := range resp.Resources {
        decodedResource, err := resource.Decode[*pbexample.Bar](r)
        if err != nil {
            return nil, err
        }
        
        // Only add Bar resources that match Foo by name. 
        if decodedResource.GetData().GetFooName() == res.Id.Name {
            result = append(result, controller.Request{ID: r.Id})
        }
    }
}
```

This approach is fine for cases when the number of `Bar` resources in a cluster is relatively small. If it's not,
then we'd be doing a large `O(N)` search on each `Bar` event which could be too expensive. 

#### Caching Mappers

For cases when `N` is too large, we'd want to use a caching layer to help us make lookups more efficient so that they
don't require an `O(N)` search of potentially all cluster resources.

The controller runtime contains a controller cache and the facilities to keep the cache up to date in response to watches. Additionally there are dependency mappers provided for querying the cache. 

_While it is possible to not use the builtin cache and manage state in dependency mappers yourself, this can get quite complex and reasoning about the correct times to track and untrack relationships is tricky to get right. Usage of the cache is therefore the advised approach._

At a high level, the controller author provides the indexes to track for each watchedtype and can then query thosfunc fooFromArgs(args ...any) ([]byte, error)e indexes in the  {
    
}future. The querying can occur during both dependency mapping and during resource reconciliation.

The following example shows how to configure the "bar" controller to rereconcile a Bar resource whenever a Foo resource is changed that references the Bar

```go
func fooReferenceFromBar(r *resource.DecodedResource[*pbexample.Bar]) (bool, []byte, error) {
    idx := index.IndexFromRefOrID(&pbresource.ID{
        Type: pbexample.FooType,
        Tenancy: r.Id.Tenancy,
        Name: r.Data.GetFooName(),
    })
    
    return true, idx, nil
}

func barController() controller.Controller {
    fooIndex := indexers.DecodedSingleIndexer(
        "foo", 
        index.ReferenceOrIDFromArgs,
        fooReferenceFromBar,
    )
	
    return controller.NewController("bar", pbexample.BarType, fooIndex).
        WithWatch(
            pbexample.FooType, 
            dependency.CacheListMapper(pbexample.BarType, fooIndex.Name()),
        ).
        WithReconciler(barReconciler{})
}
```

The controller will now reconcile Bar type resources whenever the Foo type resources they reference are updated. No further tracking is necessary as changes to all Bar types will automatically update the cache.

One limitation of the cache is that it only has knowledge about the current state of resources. That specifically means that the previous state is forgotten once the cache observes a write. This can be problematic when you want to reconcile a resource to no longer take into account something that previously reference it. 

Lets say there are two types: `Baz` and `ComputedBaz` and a controller that will aggregate all `Baz` resource with some value into a single `ComputedBaz` object. When
a `Baz` resource gets updated to no longer have a value, it should not be represented in the `ComputedBaz` resource. The typical way to work around this is to:

1. Store references to the resources that were used during reconciliation within the computed/reconciled resource. For types computed by controllers and not expected to be written directly by users a `bound_references` field should be added to the top level resource types message. For other user manageable types the references may need to be stored within the Status field.

2. Add a cache index to the watch of the computed type (usually the controllers main managed type). This index can use one of the indexers specified within the  [`internal/controller/cache/indexers`](../../../internal/controller/cache/indexers/) package. That package contains some builtin functionality around reference indexing.

3. Update the dependency mappers to query the cache index *in addition to* looking at the current state of the dependent resource. In our example above the `Baz` dependency mapper could use the [`MultiMapper`] to combine querying the cache for `Baz` types that currently should be associated with a `ComputedBaz` and querying the index added in step 2 for previous references.

### Custom Watches

In some cases, we may want to trigger reconciles for events that aren't generated from CRUD operations on resources, for example
when Envoy proxy connects or disconnects to a server. Controller-runtime allows us to setup watches from
events that come from a custom event channel. Please see [xds-controller](https://github.com/hashicorp/consul/blob/ecfeb7aac51df8730064d869bb1f2c633a531522/internal/mesh/internal/controllers/xds/controller.go#L40-L41) for examples of custom watches.

## Statuses

In many cases, controllers would need to update statuses on resources to let the user know about the successful or unsuccessful
state of a resource.

These are the guidelines that we recommend for statuses:

* While status conditions is a list, the Condition type should be treated as a key in a map, meaning a resource should not have two status conditions with the same type.
* Controllers need to both update successful and unsuccessful conditions states. This is because we need to make sure that we clear any failed status conditions.
* Status conditions should be named such that the `True` state is a successful state and `False` state is a failed state. 

## Best Practices

Below is a list of controller best practices that we've learned so far. Many of them are inspired by [kubebuilder](https://book.kubebuilder.io/reference/good-practices).

* Avoid monolithic controllers as much as possible. A single controller should only manage a single resource to avoid complexity and race conditions.
* If using cached mappers, aim to write (update or delete entries) to mappers in the `Reconcile` method and read from them in the mapper functions used by watches.
* Fetch all data in the `Reconcile` method and avoid caching it from the mapper functions. This ensures that we get the latest data for each reconciliation.
