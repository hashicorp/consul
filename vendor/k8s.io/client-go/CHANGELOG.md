TODO: This document was manually maintained so might be incomplete. The
automation effort is tracked in
https://github.com/kubernetes/client-go/issues/234.

Changes in `k8s.io/api` and `k8s.io/apimachinery` are mentioned here
because `k8s.io/client-go` depends on them.

# v6.0.0

**Breaking Changes:**

* If you upgrade your client-go libs and use the `AppsV1() or Apps()` interface, please note that the default garbage collection behavior is changed.

    * [https://github.com/kubernetes/kubernetes/pull/55148](https://github.com/kubernetes/kubernetes/pull/55148)

* Swagger 1.2 retriever `DiscoveryClient.SwaggerSchema` was removed from the discovery client

    * [https://github.com/kubernetes/kubernetes/pull/53441](https://github.com/kubernetes/kubernetes/pull/53441)

* Informers got a NewFilteredSharedInformerFactory to e.g. filter by namespace

    * [https://github.com/kubernetes/kubernetes/pull/54660](https://github.com/kubernetes/kubernetes/pull/54660)

* [k8s.io/api] The dynamic admission webhook is split into two kinds, mutating and validating. 
The kinds have changed completely and old code must be ported to `admissionregistration.k8s.io/v1beta1` - 
`MutatingWebhookConfiguration` and `ValidatingWebhookConfiguration`

    * [https://github.com/kubernetes/kubernetes/pull/55282](https://github.com/kubernetes/kubernetes/pull/55282)

* [k8s.io/api] Renamed `core/v1.ScaleIOVolumeSource` to `ScaleIOPersistentVolumeSource`

    * [https://github.com/kubernetes/kubernetes/pull/54013](https://github.com/kubernetes/kubernetes/pull/54013)

* [k8s.io/api] Renamed `core/v1.RBDVolumeSource` to `RBDPersistentVolumeSource`

    * [https://github.com/kubernetes/kubernetes/pull/54302](https://github.com/kubernetes/kubernetes/pull/54302)

* [k8s.io/api] Removed `core/v1.CreatedByAnnotation`

    * [https://github.com/kubernetes/kubernetes/pull/54445](https://github.com/kubernetes/kubernetes/pull/54445)

* [k8s.io/api] Renamed `core/v1.StorageMediumHugepages` to `StorageMediumHugePages`

    * [https://github.com/kubernetes/kubernetes/pull/54748](https://github.com/kubernetes/kubernetes/pull/54748)

* [k8s.io/api] `core/v1.Taint.TimeAdded` became a pointer

   * [https://github.com/kubernetes/kubernetes/pull/43016](https://github.com/kubernetes/kubernetes/pull/43016)

* [k8s.io/api] `core/v1.DefaultHardPodAffinitySymmetricWeight` type changed from int to int32

    * [https://github.com/kubernetes/kubernetes/pull/53850](https://github.com/kubernetes/kubernetes/pull/53850)

* [k8s.io/apimachinery] `ObjectCopier` interface was removed (requires switch to new generators with DeepCopy methods)

    * [https://github.com/kubernetes/kubernetes/pull/53525](https://github.com/kubernetes/kubernetes/pull/53525)

**New Features:**

* Certificate manager was moved from kubelet to `k8s.io/client-go/util/certificates`

   * [https://github.com/kubernetes/kubernetes/pull/49654](https://github.com/kubernetes/kubernetes/pull/49654)

* [k8s.io/api] Workloads api types are promoted to `apps/v1` version

    * [https://github.com/kubernetes/kubernetes/pull/53679](https://github.com/kubernetes/kubernetes/pull/53679)

* [k8s.io/api] Added `storage.k8s.io/v1alpha1` API group

    * [https://github.com/kubernetes/kubernetes/pull/54463](https://github.com/kubernetes/kubernetes/pull/54463)

* [k8s.io/api] Added support for conditions in StatefulSet status

    * [https://github.com/kubernetes/kubernetes/pull/55268](https://github.com/kubernetes/kubernetes/pull/55268)

* [k8s.io/api] Added support for conditions in DaemonSet status

    * [https://github.com/kubernetes/kubernetes/pull/55272](https://github.com/kubernetes/kubernetes/pull/55272)

* [k8s.io/apimachinery] Added polymorphic scale client in `k8s.io/client-go/scale`, which supports scaling of resources in arbitrary API groups

    * [https://github.com/kubernetes/kubernetes/pull/53743](https://github.com/kubernetes/kubernetes/pull/53743)

* [k8s.io/apimachinery] `meta.MetadataAccessor` got API chunking support

    * [https://github.com/kubernetes/kubernetes/pull/53768](https://github.com/kubernetes/kubernetes/pull/53768)

* [k8s.io/apimachinery] `unstructured.Unstructured` got getters and setters

    * [https://github.com/kubernetes/kubernetes/pull/51940](https://github.com/kubernetes/kubernetes/pull/51940)

**Bug fixes and Improvements:**

* The body in glog output is not truncated with log level 10

    * [https://github.com/kubernetes/kubernetes/pull/54801](https://github.com/kubernetes/kubernetes/pull/54801)

* [k8s.io/api] Unset `creationTimestamp` field is output as null if encoded from an unstructured object

    * [https://github.com/kubernetes/kubernetes/pull/53464](https://github.com/kubernetes/kubernetes/pull/53464)

* [k8s.io/apimachinery] Redirect behavior is restored for proxy subresources

    * [https://github.com/kubernetes/kubernetes/pull/52933](https://github.com/kubernetes/kubernetes/pull/52933)

* [k8s.io/apimachinery] Random string generation functions are optimized

    * [https://github.com/kubernetes/kubernetes/pull/53720](https://github.com/kubernetes/kubernetes/pull/53720)

# v5.0.1

Bug fix: picked up a security fix [kubernetes/kubernetes#53443](https://github.com/kubernetes/kubernetes/pull/53443) for `PodSecurityPolicy`.

# v5.0.0

**New features:**

* Added paging support

   * [https://github.com/kubernetes/kubernetes/pull/51876](https://github.com/kubernetes/kubernetes/pull/51876)

* Added support for client-side spam filtering of events

   * [https://github.com/kubernetes/kubernetes/pull/47367](https://github.com/kubernetes/kubernetes/pull/47367)

* Added support for http etag and caching

   * [https://github.com/kubernetes/kubernetes/pull/50404](https://github.com/kubernetes/kubernetes/pull/50404)

* Added priority queue support to informer cache

   * [https://github.com/kubernetes/kubernetes/pull/49752](https://github.com/kubernetes/kubernetes/pull/49752)

* Added openstack auth provider

   * [https://github.com/kubernetes/kubernetes/pull/39587](https://github.com/kubernetes/kubernetes/pull/39587)

* Added metrics for checking reflector health

   * [https://github.com/kubernetes/kubernetes/pull/48224](https://github.com/kubernetes/kubernetes/pull/48224)

* Client-go now includes the leaderelection package

   * [https://github.com/kubernetes/kubernetes/pull/39173](https://github.com/kubernetes/kubernetes/pull/39173)

**API changes:**

* Promoted Autoscaling v2alpha1 to v2beta1

   * [https://github.com/kubernetes/kubernetes/pull/50708](https://github.com/kubernetes/kubernetes/pull/50708)

* Promoted CronJobs to batch/v1beta1

   * [https://github.com/kubernetes/kubernetes/pull/41901](https://github.com/kubernetes/kubernetes/pull/41901)

* Promoted rbac.authorization.k8s.io/v1beta1 to rbac.authorization.k8s.io/v1

   * [https://github.com/kubernetes/kubernetes/pull/49642](https://github.com/kubernetes/kubernetes/pull/49642)

* Added a new API version apps/v1beta2

   * [https://github.com/kubernetes/kubernetes/pull/48746](https://github.com/kubernetes/kubernetes/pull/48746)

* Added a new API version scheduling/v1alpha1

   * [https://github.com/kubernetes/kubernetes/pull/48377](https://github.com/kubernetes/kubernetes/pull/48377)

**Breaking changes:**

* Moved pkg/api and pkg/apis to [k8s.io/api](https://github.com/kubernetes/api). Other kubernetes repositories also import types from there, so they are composable with client-go.

* Removed helper functions in pkg/api and pkg/apis. They are planned to be exported in other repos. The issue is tracked [here](https://github.com/kubernetes/kubernetes/issues/48209#issuecomment-314537745). During the transition, you'll have to copy the helper functions to your projects.

* The discovery client now fetches the protobuf encoded OpenAPI schema and returns `openapi_v2.Document`

   * [https://github.com/kubernetes/kubernetes/pull/46803](https://github.com/kubernetes/kubernetes/pull/46803)

* Enforced explicit references to API group client interfaces in clientsets to avoid ambiguity.

   * [https://github.com/kubernetes/kubernetes/pull/49370](https://github.com/kubernetes/kubernetes/pull/49370)

* The generic RESTClient type (`k8s.io/client-go/rest`) no longer exposes `LabelSelectorParam` or `FieldSelectorParam` methods - use `VersionedParams` with `metav1.ListOptions` instead. The `UintParam` method has been removed. The `timeout` parameter will no longer cause an error when using `Param()`.

   * [https://github.com/kubernetes/kubernetes/pull/48991](https://github.com/kubernetes/kubernetes/pull/48991)

# v4.0.0

No significant changes since v4.0.0-beta.0.

# v4.0.0-beta.0

**New features:**

* Added OpenAPISchema support in the discovery client

    * [https://github.com/kubernetes/kubernetes/pull/44531](https://github.com/kubernetes/kubernetes/pull/44531)

* Added mutation cache filter: MutationCache is able to take the result of update operations and stores them in an LRU that can be used to provide a more current view of a requested object.

    * [https://github.com/kubernetes/kubernetes/pull/45838](https://github.com/kubernetes/kubernetes/pull/45838/commits/f88c7725b4f9446c652d160bdcfab7c6201bddea)

* Moved the remotecommand package (used by `kubectl exec/attach`) to client-go

    * [https://github.com/kubernetes/kubernetes/pull/41331](https://github.com/kubernetes/kubernetes/pull/41331)

* Added support for following redirects to the SpdyRoundTripper

    * [https://github.com/kubernetes/kubernetes/pull/44451](https://github.com/kubernetes/kubernetes/pull/44451)

* Added Azure Active Directory plugin

    * [https://github.com/kubernetes/kubernetes/pull/43987](https://github.com/kubernetes/kubernetes/pull/43987)

**Usability improvements:**

* Added several new examples and reorganized client-go/examples

    * [Related PRs](https://github.com/kubernetes/kubernetes/commits/release-1.7/staging/src/k8s.io/client-go/examples)

**API changes:**

* Added networking.k8s.io/v1 API

    * [https://github.com/kubernetes/kubernetes/pull/39164](https://github.com/kubernetes/kubernetes/pull/39164)

* ControllerRevision type added for StatefulSet and DaemonSet history.

    * [https://github.com/kubernetes/kubernetes/pull/45867](https://github.com/kubernetes/kubernetes/pull/45867)

* Added support for initializers

    * [https://github.com/kubernetes/kubernetes/pull/38058](https://github.com/kubernetes/kubernetes/pull/38058)

* Added admissionregistration.k8s.io/v1alpha1 API

    * [https://github.com/kubernetes/kubernetes/pull/46294](https://github.com/kubernetes/kubernetes/pull/46294)

**Breaking changes:**

* Moved client-go/util/clock to apimachinery/pkg/util/clock 

    * [https://github.com/kubernetes/kubernetes/pull/45933](https://github.com/kubernetes/kubernetes/pull/45933/commits/8013212db54e95050c622675c6706cce5de42b45)

* Some [API helpers](https://github.com/kubernetes/client-go/blob/release-3.0/pkg/api/helpers.go) were removed. 

* Dynamic client takes GetOptions as an input parameter

    * [https://github.com/kubernetes/kubernetes/pull/47251](https://github.com/kubernetes/kubernetes/pull/47251)

**Bug fixes:**

* PortForwarder: don't log an error if net.Listen fails. [https://github.com/kubernetes/kubernetes/pull/44636](https://github.com/kubernetes/kubernetes/pull/44636)

* oidc auth plugin not to override the Auth header if it's already exits. [https://github.com/kubernetes/kubernetes/pull/45529](https://github.com/kubernetes/kubernetes/pull/45529)

* The --namespace flag is now honored for in-cluster clients that have an empty configuration. [https://github.com/kubernetes/kubernetes/pull/46299](https://github.com/kubernetes/kubernetes/pull/46299)

* GCP auth plugin no longer overwrites existing Authorization headers. [https://github.com/kubernetes/kubernetes/pull/45575](https://github.com/kubernetes/kubernetes/pull/45575)

# v3.0.0

Bug fixes:
* Use OS-specific libs when computing client User-Agent in kubectl, etc. (https://github.com/kubernetes/kubernetes/pull/44423)
* kubectl commands run inside a pod using a kubeconfig file now use the namespace specified in the kubeconfig file, instead of using the pod namespace. If no kubeconfig file is used, or the kubeconfig does not specify a namespace, the pod namespace is still used as a fallback. (https://github.com/kubernetes/kubernetes/pull/44570)
* Restored the ability of kubectl running inside a pod to consume resource files specifying a different namespace than the one the pod is running in. (https://github.com/kubernetes/kubernetes/pull/44862)

# v3.0.0-beta.0

* Added dependency on k8s.io/apimachinery. The impacts include changing import path of API objects like `ListOptions` from `k8s.io/client-go/pkg/api/v1` to `k8s.io/apimachinery/pkg/apis/meta/v1`.
* Added generated listers (listers/) and informers (informers/)
* Kubernetes API changes:
  * Added client support for:
    * authentication/v1
    * authorization/v1
    * autoscaling/v2alpha1
    * rbac/v1beta1
    * settings/v1alpha1
    * storage/v1
  * Changed client support for:
    * certificates from v1alpha1 to v1beta1
    * policy from v1alpha1 to v1beta1
  * Deleted client support for:
    * extensions/v1beta1#Job
* CHANGED: pass typed options to dynamic client (https://github.com/kubernetes/kubernetes/pull/41887)

# v2.0.0

* Included bug fixes in k8s.io/kuberentes release-1.5 branch, up to commit 
  bde8578d9675129b7a2aa08f1b825ec6cc0f3420

# v2.0.0-alpha.1

* Removed top-level version folder (e.g., 1.4 and 1.5), switching to maintaining separate versions
  in separate branches.
* Clientset supported multiple versions per API group
* Added ThirdPartyResources example
* Kubernetes API changes
  * Apps API group graduated to v1beta1 
  * Policy API group graduated to v1beta1
  * Added support for batch/v2alpha1/cronjob
  * Renamed PetSet to StatefulSet
  

# v1.5.0

* Included the auth plugin (https://github.com/kubernetes/kubernetes/pull/33334)
* Added timeout field to RESTClient config (https://github.com/kubernetes/kubernetes/pull/33958)
