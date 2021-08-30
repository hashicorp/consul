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

### Reporting an Issue:
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
   linked.

5. The issue is closed.

## Building Consul

If you wish to work on Consul itself, you'll first need [Go](https://golang.org)
installed (The version of Go should match the one of our [CI config's](https://github.com/hashicorp/consul/blob/main/.circleci/config.yml) Go image).


Next, clone this repository and then run `make dev`. In a few moments, you'll have a working
`consul` executable in `consul/bin` and `$GOPATH/bin`:

>Note: `make dev` will build for your local machine's os/architecture. If you wish to build for all os/architecture combinations use `make`.

## Making Changes to Consul

The first step to making changes is to fork Consul. Afterwards, the easiest way 
to work on the fork is to set it as a remote of the Consul project:

1. Navigate to `$GOPATH/src/github.com/hashicorp/consul`
2. Rename the existing remote's name: `git remote rename origin upstream`.
3. Add your fork as a remote by running
   `git remote add origin <github url of fork>`. For example:
   `git remote add origin https://github.com/myusername/consul`.
4. Checkout a feature branch: `git checkout -t -b new-feature`
5. Make changes
6. Push changes to the fork when ready to submit PR:
   `git push -u origin new-feature`

By following these steps you can push to your fork to create a PR, but the code on disk still
lives in the spot where the go cli tools are expecting to find it.

>Note: If you make any changes to the code, run `gofmt -s -w` to automatically format the code according to Go standards.

## Testing

During development, it may be more convenient to check your work-in-progress by running only the tests which you expect to be affected by your changes, as the full test suite can take several minutes to execute. [Go's built-in test tool](https://golang.org/pkg/cmd/go/internal/test/) allows specifying a list of packages to test and the `-run` option to only include test names matching a regular expression.
The `go test -short` flag can also be used to skip slower tests.

Examples (run from the repository root):
- `go test -v ./connect` will run all tests in the connect package (see `./connect` folder)
- `go test -v -run TestRetryJoin ./command/agent` will run all tests in the agent package (see `./command/agent` folder) with name substring `TestRetryJoin`

When a pull request is opened CI will run all tests and lint to verify the change.

## Go Module Dependencies

If a dependency is added or change, run `go mod tidy` to update `go.mod` and `go.sum`.

## Developer Documentation

Documentation about the Consul code base is under [./contributing],
and godoc package document can be read at [pkg.go.dev/github.com/hashicorp/consul].

[./contributing]: ../contributing/README.md
[pkg.go.dev/github.com/hashicorp/consul]: https://pkg.go.dev/github.com/hashicorp/consul

### Checklists

Some common changes that many PRs require such as adding config fields, are
documented through checklists.

Please check in `contributing/` for any `checklist-*.md` files that might help
with your change.
