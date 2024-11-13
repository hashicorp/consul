```mermaid
flowchart TD
  demo/v1/album
  demo/v1/artist
  demo/v1/concept
  demo/v1/executive
  demo/v1/recordlabel
  demo/v2/album
  demo/v2/artist
  internal/v1/tombstone
  multicluster/v2/computedexportedservices --> multicluster/v2/exportedservices
  multicluster/v2/computedexportedservices --> multicluster/v2/namespaceexportedservices
  multicluster/v2/computedexportedservices --> multicluster/v2/partitionexportedservices
  multicluster/v2/exportedservices
  multicluster/v2/namespaceexportedservices
  multicluster/v2/partitionexportedservices
```
