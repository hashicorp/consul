# Resource and Controller Developer Guide

This is a whistle-stop tour through adding a new resource type and controller to
Consul 🚂

## Resource Schema

Adding a new resource type begins with defining the object schema as a protobuf
message, in the appropriate package under [`proto-public`](../../../proto-public).

```shell
$ mkdir proto-public/pbfoo/v1alpha1
```

```proto
// proto-public/pbfoo/v1alpha1/foo.proto
syntax = "proto3";

import "pbresource/resource.proto";
import "pbresource/annotations.proto";

package hashicorp.consul.foo.v1alpha1;

message Bar {
  option (hashicorp.consul.resource.spec) = {scope: SCOPE_NAMESPACE};
  
  string baz = 1;
  hashicorp.consul.resource.ID qux = 2;
}
```

```shell
$ make proto
```

Next, we must add our resource type to the registry. At this point, it's useful
to add a package (e.g. under [`internal`](../../../internal)) to contain the logic
associated with this resource type.

The convention is to have this package export variables for its type identifiers
along with a method for registering its types:

```Go
// internal/foo/types.go
package foo

import (
	"github.com/hashicorp/consul/internal/resource"
	pbv1alpha1 "github.com/hashicorp/consul/proto-public/pbfoo/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterTypes(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbv1alpha1.BarType, 
		Scope: resource.ScopePartition,
		Proto: &pbv1alpha1.Bar{},
	})
}
```
Note that Scope reference the scope of the new resource, `resource.ScopePartition` 
mean that resource will be at the partition level and have no namespace, while `resource.ScopeNamespace` mean it will have both a namespace 
and a partition.

Update the `NewTypeRegistry` method in [`type_registry.go`] to call your
package's type registration method:

[`type_registry.go`]: ../../../agent/consul/type_registry.go

```Go
import (
	// …
	"github.com/hashicorp/consul/internal/foo"
	// …
)

func NewTypeRegistry() resource.Registry {
	// …
    foo.RegisterTypes(registry)
	// …
}
```

That should be all you need to start using your new resource type. Test it out
by starting an agent in dev mode:

```shell
$ make dev
$ consul agent -dev
```

You can now use [grpcurl](https://github.com/fullstorydev/grpcurl) to interact
with the [resource service](../../../proto-public/pbresource/resource.proto):

```shell
$ grpcurl -d @ \
  -plaintext \
  -protoset pkg/consul.protoset \
  127.0.0.1:8502 \
  hashicorp.consul.resource.ResourceService.Write \
<<EOF
  {
    "resource": {
      "id": {
        "type": {
          "group": "foo",
          "group_version": "v1alpha1",
          "kind": "bar"
        },
        "tenancy": {
          "partition": "default",
          "namespace": "default"
        }
      },
      "data": {
        "@type": "types.googleapis.com/hashicorp.consul.foo.v1alpha1.Bar",
        "baz": "Hello World"
      }
    }
  }
EOF
```

## Validation

Broadly, there are two kinds of validation you might want to perform against
your resources:

- **Structural** validation ensures the user's input is well-formed, for
  example: checking that a required field is provided, or that a port is within
  an acceptable range.
- **Semantic** validation ensures that the resource makes sense in the context
  of *other* resources, for example: checking that an L7 intention is not
  targeting an L4 service.

Structural validation should be done up-front, before the resource is admitted,
using a validation hook provided in the type registration:

```Go
func RegisterTypes(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbv1alpha1.BarType,
		Proto:    &pbv1alpha1.Bar{}, 
		Scope:    resource.ScopeNamespace,
		Validate: validateBar,
	})
}

func validateBar(res *pbresource.Resource) error {
	var bar pbv1alpha1.Bar
	if err := res.Data.UnmarshalTo(&bar); err != nil {
		return resource.NewErrDataParse(&bar, err)
	}
	if bar.Baz == "" {
		return resource.ErrInvalidField{
			Name:    "baz",
			Wrapped: resource.ErrMissing,
		}
	}
	return nil
}
```

Semantic validation should be done asynchronously, after the resource is
written, by controllers ([covered below](#controllers)).

## Authorization

You can control how operations on your resource type are authorized by providing
a set of ACL hooks:

```Go
func RegisterTypes(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbv1alpha1.BarType,
		Proto: &pbv1alpha1.Bar{}, 
		Scope: resource.ScopeNamespace,
		ACLs: &resource.ACLHooks{,
			Read:  authzReadBar,
			Write: authzWriteBar,
			List:  authzListBar,
		},
	})
}

func authzReadBar(authz acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID,  _ *pbresource.Resource) error {
	return authz.ToAllowAuthorizer().
		BarReadAllowed(id.Name, authzContext)
}

func authzWriteBar(authz acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	return authz.ToAllowAuthorizer().
		BarWriteAllowed(res.ID().Name, authzContext)
}

func authzListBar(authz acl.Authorizer, authzContext *acl.AuthorizerContext) error {
	return authz.ToAllowAuthorizer().
		BarListAllowed(authzContext)
}
```

If you do not provide ACL hooks, `operator:read` and `operator:write`
permissions will be required.

## Mutation

Sometimes, it's necessary to modify resources before they're persisted. For
example, to set sensible default values or normalize user input. You can do this
by providing a mutation hook:

```Go
func RegisterTypes(r resource.Registry) {
	r.Register(resource.Registration{
		Type:   pbv1alpha1.BarType,
		Proto:  &pbv1alpha1.Bar{}, 
		Scope:  resource.ScopeNamespace,
		Mutate: mutateBar,
	})
}

func mutateBar(res *pbresource.Resource) error {
	var bar pbv1alpha1.Bar
	if err := res.Data.UnmarshalTo(&bar); err != nil {
		return resource.NewErrDataParse(&bar, err)
	}
	bar.Baz = strings.ToLower(bar.Baz)
	return res.Data.MarshalFrom(&bar)
}
```

## Controllers

Controllers are where the business logic of your resources will live. They're
asynchronous [reconciliation loops] that "wake up" whenever a resource is
modified to validate and realize the changes.

You can create a new controller using the [builder API]. Start by identifying
the resource type you want this controller to manage, and provide a reconciler
that will be called whenever a resource of that type is changed.

```Go
package foo

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	pbv1alpha1 "github.com/hashicorp/consul/proto-public/pbfoo/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func barController() controller.Controller {
	return controller.NewController("bar", pbv1alpha1.BarType).
		WithReconciler(barReconciler{})
}

type barReconciler struct{}

func (barReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: req.ID})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil
	case err != nil:
		return err
	}

	var bar pbv1alpha1.Bar
	if err := rsp.Resource.Data.UnmarshalTo(&bar); err != nil {
		return err
	}
	rt.Logger.Debug("Hello from bar reconciler!", "baz", bar.Baz)

	return nil
}
```

[reconciliation loops]: https://www.oreilly.com/library/view/97-things-every/9781492050896/ch73.html
[builder API]: https://pkg.go.dev/github.com/hashicorp/consul/internal/controller#Controller

Next, register your controller with the controller manager. Another common
pattern is to have your package expose a method for registering controllers,
which is called from `registerControllers` in [`server.go`].

[`server.go`]: ../../../agent/consul/server.go

```Go
package foo

func RegisterControllers(mgr *controller.Manager) {
	mgr.Register(barController())
}
```

```Go
package consul

func (s *Server) registerControllers() {
	// …
	foo.RegisterControllers(s.controllerManager)
	// …
}
```

### Retries

By default, if your reconciler returns an error, it will be retried with
exponential backoff. While this is correct in most circumstances, you can
override it by returning [`RequeueAfter`] or [`RequeueNow`].

[`RequeueAfter`]: https://pkg.go.dev/github.com/hashicorp/consul/internal/controller#RequeueAfter
[`RequeueNow`]: https://pkg.go.dev/github.com/hashicorp/consul/internal/controller#RequeueNow

```Go
func (barReconciler) Reconcile(context.Context, controller.Runtime, controller.Request) error {
	if time.Now().Hour() < 9 {
		return controller.RequeueAfter(1 * time.Hour)
	}
	return nil
}
```

### Status

Controllers can communicate the result of reconciling resource changes (e.g.
surfacing semantic validation issues) with users and other controllers by
updating the resource's status using the `WriteStatus` method.

Each resource can have multiple statuses, typically one per controller,
identified by a string key. Statuses are composed of a set of conditions, which
represent discreet observations about the resource in relation to the current
state of the system.

That all sounds a little abstract, so let's take a look at an example.

```Go
client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
	Id:     res.Id,
	Key:    "consul.io/bar",
	Status: &pbresource.Status{
		ObservedGeneration: res.Generation,
		Conditions: []*pbresource.Condition{
			{
				Type:    "Healthy",
				State:   pbresource.Condition_STATE_TRUE,
				Reason:  "OK",
				Message: "All checks are passing",
			},
			{
				Type:    "ResolvedRefs",
				State:   pbresource.Condition_STATE_FALSE,
				Reason:  "INVALID_REFERENCE",
				Message: "Bar contained an invalid reference to qux",
				Resource: resource.Reference(bar.Qux, ""),
			},
		},
	},
})
```

In the previous example, the controller makes two observations about the
current state of the resource:

1. That it's "healthy" (whatever that means in this hypothetical scenario)
1. That it contains a reference that couldn't be resolved

The `Type` and `Reason` should be simple, machine-readable, strings, but there
aren't any strict rules about what are acceptable values. Over time, we
anticipate that common values will emerge that we'll standardize on for
consistency.

`Message` should be a human-readable explanation of the condition.

> **Warning**  
> Writing a status to the resource will cause it to be re-reconciled. To avoid
> infinite loops, we recommend dirty checking the status before writing it with
> [`resource.EqualStatus`].

[`resource.EqualStatus`]: https://pkg.go.dev/github.com/hashicorp/consul/internal/resource#EqualStatus

### Watching Other Resources

In addition to watching their "managed" resources, controllers can also watch
resources of different, related, types. For example, the service endpoints
controller also watches workloads and services.

```Go
func barController() controller.Controller {
	return controller.NewController("bar", pbv1alpha1.BarType).
		WithWatch(pbv1alpha1.BazType, controller.MapOwner)
		WithReconciler(barReconciler{})
}
```

The second argument to `WithWatch` is a [dependency mapper] function. Whenever a
resource of the watched type is modified, the dependency mapper will be called
to determine which of the controller's managed resources need to be reconciled.

[`dependency.MapOwner`] is a convenience function which causes the watched
resource's [owner](#ownership--cascading-deletion) to be reconciled.

[dependency mapper]: https://pkg.go.dev/github.com/hashicorp/consul/internal/controller#DependencyMapper
[`dependency.MapOwner`]: https://pkg.go.dev/github.com/hashicorp/consul/internal/controller/dependency#MapOwner

### Placement

By default, only a single, leader-elected, replica of each controller will run
within a cluster. Sometimes it's necessary to override this, for example when
you want to run a copy of the controller on each server (e.g. to apply some
configuration to the server whenever it changes). You can do this by changing
the controller's placement.

```Go
func barController() controller.Controller {
	return controller.NewController("bar", pbv1alpha1.BarType).
		WithPlacement(controller.PlacementEachServer)
		WithReconciler(barReconciler{})
}
```

> **Warning**  
> Controllers placed with [`controller.PlacementEachServer`] generally shouldn't
> modify resources (as it could lead to race conditions).

[`controller.PlacementEachServer`]: https://pkg.go.dev/github.com/hashicorp/consul/internal/controller#PlacementEachServer

### Initializer

If your controller needs to execute setup steps when the controller
first starts and before any resources are reconciled, you can add an
Initializer.

If the controller has an Initializer, it will not start unless the
Initialize method is successful. The controller does not have retry
logic for the initialize method specifically, but the controller
is restarted on error. When restarted, the controller will attempt
to execute the initialization again.

The example below has the controller creating a default resource as
part of initialization.

```Go
package foo

import (
 "context"

 "github.com/hashicorp/consul/internal/controller"
 pbv1alpha1 "github.com/hashicorp/consul/proto-public/pbfoo/v1alpha1"
 "github.com/hashicorp/consul/proto-public/pbresource"
)

func barController() controller.Controller {
 return controller.ForType(pbv1alpha1.BarType).
  WithReconciler(barReconciler{}).
  WithInitializer(barInitializer{})
}

type barInitializer struct{}

func (barInitializer) Initialize(ctx context.Context, rt controller.Runtime) error {
  _, err := rt.Client.Write(ctx,
    &pbresource.WriteRequest{
    Resource: &pbresource.Resource{
     Id: &pbresource.ID{
      Name: "default",
      Type: pbv1alpha1.BarType,
     },
    },
   },
  )
  if err != nil {
   return err
  }

  return nil
}
```

### Finalizer

A finalizer allows a controller to execute teardown logic before a
resource is deleted. This can be useful to perform cleanup or block
deletion until certain conditions are met.

Finalizers are encoded as keys within a resource's metadata map. It
is the responsibility of each controller that adds a finalizer to a
resource to remove the finalizer when it is marked for deletion.
Once a resource has no finalizers present, it is deleted by the
resource service.

When the `Delete` endpoint is called on a resource with one or more
finalizers, the resource is marked for deletion by adding an immutable
`deletionTimestamp` key to the resource's metadata map. The resource is
now effectively frozen and will only accept subsequent `Write`s
that remove finalizers. `WriteStatus` is still allowed.

The `resource` package API can be used to manage finalizers and
check whether a resource has been marked for deletion. You would
typically use this API within the logic of your controller's
`Reconcile` method to either put a finalizer in place or perform
cleanup and then remove a finalizer. Don't forget to `Write` your
changes once you add or remove finalizers.

```Go
package resource

// IsMarkedForDeletion returns true if a resource has been marked for deletion,
// false otherwise.
func IsMarkedForDeletion(res *pbresource.Resource) bool { ... }

// HasFinalizers returns true if a resource has one or more finalizers, false otherwise.
func HasFinalizers(res *pbresource.Resource) bool { ... }

// HasFinalizer returns true if a resource has a given finalizer, false otherwise.
func HasFinalizer(res *pbresource.Resource, finalizer string) bool { ... }

// AddFinalizer adds a finalizer to the given resource.
func AddFinalizer(res *pbresource.Resource, finalizer string) { ... }

// RemoveFinalizer removes a finalizer from the given resource.
func RemoveFinalizer(res *pbresource.Resource, finalizer string) { ... }

// GetFinalizers returns the set of finalizers for the given resource.
func GetFinalizers(res *pbresource.Resource) mapset.Set[string] { ... }
```

Example flow in a controller's `Reconcile` method
```Go
const finalizer = "consul.io/bar-finalizer"

func (barReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	...
	// Check if resource is marked for deletion. If yes, perform cleanup, remove finalizer, and Write the resource
	if resource.IsMarkedForDeletion(res) {
		// Perform some cleanup...
		return EnsureFinalizerRemoved(ctx, rt, res, finalizer)
	}

	// Check if resource has finalizer. If not, add it and Write the resource
	if err := EnsureHasFinalizer(ctx, rt, res, finalizer); err != nil {
		return err
	}
}
```

## Ownership & Cascading Deletion

The resource service implements a lightweight `1:N` ownership model where, on
creation, you can mark a resource as being "owned" by another resource. When the
owner is deleted, the owned resource will be deleted too.

```Go
client.Write(ctx, &pbresource.WriteRequest{
	Resource: &pbresource.Resource{,
		Owner: ownerID,
		// …
	},
})
```

## Testing

Now that you have created your controller its time to test it. The types of tests each controller should have and boiler plat for test files is documented [here](./testing.md)
