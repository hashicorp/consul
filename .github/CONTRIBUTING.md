# Contributing to Consul

>**Note:** We take Consul's security and our users' trust very seriously.
>If you believe you have found a security issue in Consul, please responsibly
>disclose by contacting us at security@hashicorp.com.

**First:** if you're unsure or afraid of _anything_, just ask or submit the
issue or pull request anyways. You won't be yelled at for giving your best
effort. The worst that can happen is that you'll be politely asked to change
something. We appreciate any sort of contributions, and don't want a wall of
rules to get in the way of that.

That said, if you want to ensure that a pull request is likely to be merged, 
talk to us! A great way to do this is in issues themselves. When you want to 
work on an issue, comment on it first and tell us the approach you want to take.

## Getting Started

### Some Ways to Contribute

* Report potential bugs.
* Suggest product enhancements.
* Increase our test coverage.
* Fix a [bug](https://github.com/hashicorp/consul/labels/type/bug).
* Implement a requested [enhancement](https://github.com/hashicorp/consul/labels/type/enhancement).
* Improve our guides and documentation. Consul's [Guides](https://www.consul.io/docs/guides/index.html), [Docs](https://www.consul.io/docs/index.html), and [api godoc](https://godoc.org/github.com/hashicorp/consul/api)
are deployed from this repo.
* Respond to questions about usage on the issue tracker or the Consul section of the [HashiCorp forum]: (https://discuss.hashicorp.com/c/consul)

### Reporting an Issue

>Note: Issues on GitHub for Consul are intended to be related to bugs or feature requests. 
>Questions should be directed to other community resources such as the: [Discuss Forum](https://discuss.hashicorp.com/c/consul/29), [FAQ](https://www.consul.io/docs/faq.html), or [Guides](https://www.consul.io/docs/guides/index.html).

* Make sure you test against the latest released version. It is possible we 
already fixed the bug you're experiencing. However, if you are on an older 
version of Consul and feel the issue is critical, do let us know.

* Check existing issues (both open and closed) to make sure it has not been 
reported previously.

* Provide a reproducible test case. If a contributor can't reproduce an issue, 
then it dramatically lowers the chances it'll get fixed.

* Aim to respond promptly to any questions made by the Consul team on your 
issue. Stale issues will be closed.

### Issue Lifecycle

1. The issue is reported.

2. The issue is verified and categorized by a Consul maintainer.
   Categorization is done via tags. For example, bugs are tagged as "bug".

3. Unless it is critical, the issue is left for a period of time (sometimes many
   weeks), giving outside contributors a chance to address the issue.

4. The issue is addressed in a pull request or commit. The issue will be
   referenced in the commit message so that the code that fixes it is clearly
   linked. Any change a Consul user might need to know about will include a
   changelog entry in the PR.

5. The issue is closed.

## Making Changes to Consul

### Prerequisites

If you wish to work on Consul itself, you'll first need to:
- install [Go](https://golang.org)
- [fork the Consul repo](../docs/contributing/fork-the-project.md)

### Building Consul

To build Consul, run `make dev`. In a few moments, you'll have a working
`consul` executable in `consul/bin` and `$GOPATH/bin`:

>Note: `make dev` will build for your local machine's os/architecture. If you wish to build for all os/architecture combinations, use `make`.

### Modifying the Code

#### Code Formatting

Go provides [tooling to apply consistent code formatting](https://golang.org/doc/effective_go#formatting).
If you make any changes to the code, run `gofmt -s -w` to automatically format the code according to Go standards.

##### Organizing Imports

Group imports using `goimports -local github.com/hashicorp/consul/` to keep [local packages](https://github.com/golang/tools/commit/ed69e84b1518b5857a9f4e01d1f9cefdcc45246e) in their own section.

Example: 
```
import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
)
```

#### Updating Go Module Dependencies

If a dependency is added or change, run `go mod tidy` to update `go.mod` and `go.sum`.

#### Developer Documentation

Developer-focused documentation about the Consul code base is under [./docs],
and godoc package document can be read at [pkg.go.dev/github.com/hashicorp/consul].

[./docs]: ../docs/README.md
[pkg.go.dev/github.com/hashicorp/consul]: https://pkg.go.dev/github.com/hashicorp/consul

### Testing

During development, it may be more convenient to check your work-in-progress by running only the tests which you expect to be affected by your changes, as the full test suite can take several minutes to execute. [Go's built-in test tool](https://golang.org/pkg/cmd/go/internal/test/) allows specifying a list of packages to test and the `-run` option to only include test names matching a regular expression.
The `go test -short` flag can also be used to skip slower tests.

Examples (run from the repository root):
- `go test -v ./connect` will run all tests in the connect package (see `./connect` folder)
- `go test -v -run TestRetryJoin ./command/agent` will run all tests in the agent package (see `./command/agent` folder) with name substring `TestRetryJoin`

When a pull request is opened CI will run all tests and lint to verify the change.

### Submitting a Pull Request

Before writing any code, we recommend:
- Create a Github issue if none already exists for the code change you'd like to make.
- Write a comment on the Github issue indicating you're interested in contributing so
maintainers can provide their perspective if needed.

Keep your pull requests (PRs) small and open them early so you can get feedback on
approach from maintainers before investing your time in larger changes. For example,
see how [applying URL-decoding of resource names across the whole HTTP API](https://github.com/hashicorp/consul/issues/11258)
started with [iterating on the right approach for a few endpoints](https://github.com/hashicorp/consul/pull/11335)
before applying more broadly.

When you're ready to submit a pull request:
1. Review the [list of checklists](#checklists) for common changes and follow any
   that apply to your work.
2. Include evidence that your changes work as intended (e.g., add/modify unit tests;
   describe manual tests you ran, in what environment,
   and the results including screenshots or terminal output).
3. Open the PR from your fork against base repository `hashicorp/consul` and branch `main`.
   - [Link the PR to its associated issue](https://docs.github.com/en/issues/tracking-your-work-with-issues/linking-a-pull-request-to-an-issue).
4. Include any specific questions that you have for the reviewer in the PR description
   or as a PR comment in Github.
   - If there's anything you find the need to explain or clarify in the PR, consider
   whether that explanation should be added in the source code as comments.
   - You can submit a [draft PR](https://github.blog/2019-02-14-introducing-draft-pull-requests/)
   if your changes aren't finalized but would benefit from in-process feedback.
5. If there's any reason Consul users might need to know about this change,
   [add a changelog entry](../docs/contributing/add-a-changelog-entry.md).
6. Add labels to your pull request. A table of commonly use labels is below. 
   If you have any questions about which to apply, feel free to call it out in the PR or comments.
   | Label | When to Use |
   | --- | --- |
   | `pr/no-changelog` | This PR does not have an intended changelog entry |
   | `pr/no-metrics-test` | This PR does not require any testing for metrics |
   | `backport/stable-website` | This PR contains documentation changes that are ready to be deployed immediately. Changes will also automatically get backported to the latest release branch |
   | `backport/1.12.x` | Backport the changes in this PR to the targeted release branch. Consult the [Consul Release Notes](https://www.consul.io/docs/release-notes) page to view active releases. |
   Other labels may automatically be added by the Github Action CI.
7. After you submit, the Consul maintainers team needs time to carefully review your
   contribution and ensure it is production-ready, considering factors such as: security,
   backwards-compatibility, potential regressions, etc.
8. After you address Consul maintainer feedback and the PR is approved, a Consul maintainer
   will merge it. Your contribution will be available from the next major release (e.g., 1.x)
   unless explicitly backported to an existing or previous major release by the maintainer.
9. Any backport labels will generate an additional PR to the targeted release branch. 
   These will be linked in the original PR.
   Assuming the tests pass, the PR will be merged automatically. 
   If the tests fail, it is you responsibility to resolve the issues with backports and request another reviewer.

#### Checklists

Some common changes that many PRs require are documented through checklists as
`checklist-*.md` files in [docs/](../docs/), including:
- [Adding config fields](../docs/config/checklist-adding-config-fields.md)
