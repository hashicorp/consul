# changes

- `/intro/getting-started/install` redirects to `/intro/getting-started`
- `/guides/hcl` folders with only index changed to named individual files
- `/docs/install/index.html` to `docs/install.html`
- `/docs/other` dumped to root
- `/docs/basics/terminology` -> `/docs/terminology`
- `/docs/configuration/from-1.5` -> `/docs/from-1.5`
- `/docs/from-1.5/functions.html` -> `/docs/from-1.5/functions/index.html`
- [BREAKING] `/docs/from-1.5/functions/collection/index.html` -> `/docs/from-1.5/functions/collection/index-fn.html`
- `/docs/from-1.5/functions/*/overview.html` -> `/docs/from-1.5/functions/*/index.html`
- `/docs/builders/amazon-*` -> `/docs/builders/amazon/*`
- `/docs/builders/azure-*` -> `/docs/builders/azure/*`
- `/docs/builders/hyperv-*` -> `/docs/builders/hyperv/*`
- `/docs/builders/oracle-*` -> `/docs/builders/oracle/*`
- `/docs/builders/osc-*` -> `/docs/builders/outscale/*`
- `/docs/builders/outscale.html` -> `/docs/builders/outscale/index.html`
- `/docs/builders/parallels-*` -> `/docs/builders/parallels/*`
- `/docs/builders/virtualbox-*` -> `/docs/builders/virtualbox/*`
- `/docs/builders/vmware-*` -> `/docs/builders/vmware/*`
- `/docs/builders/vsphere-*` -> `/docs/builders/vmware/vsphere-*`

# notes:

- empty index files on all `from-1.5/functions/*`
- how do the generated docs work? can we keep it even with changes?
- should any of the other builders be nested under a subdirectory? like `alicloud-ecs`, `tencent-cvm` or `ucloud-uhost`?
