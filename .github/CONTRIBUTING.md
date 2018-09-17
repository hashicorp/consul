# Contributing to Consul
>**Note:** We take Consul's security and our users' trust very seriously.
>If you believe you have found a security issue in Consul, please responsibly
>disclose by contacting us at security@hashicorp.com.

**First:** if you're unsure or afraid of _anything_, just ask or submit the
issue or pull request anyways. You won't be yelled at for giving your best
effort. The worst that can happen is that you'll be politely asked to change
something. We appreciate any sort of contributions, and don't want a wall of
rules to get in the way of that.

That said, if you want to ensure that a pull request is likely to be merged, talk to us! 
A great way to do this is in issues themselves. When you want to work on an issue, comment on it first 
and tell us the approach you want to take.

## Getting Started
### Some Ways to Contribute
* Report potential bugs.
* Suggest product enhancements.
* Improve our unit test coverage.
* Fix a [bug](https://github.com/hashicorp/consul/labels/bug).
* Implement a requested [enhancement](https://github.com/hashicorp/consul/labels/enhancement).
* Improve our guides and documentation. Consul's [Guides](https://www.consul.io/docs/guides/index.html), [Docs](https://www.consul.io/docs/index.html), and [api godoc](https://godoc.org/github.com/hashicorp/consul/api)
are hosted in this repo.

### Reporting an Issue:
>Note: Issues on GitHub for Consul are intended to be related to bugs or feature requests. 
>Questions should be directed to other community resources such as the: [Mailing List](https://groups.google.com/group/consul-tool/), [FAQ](https://www.consul.io/docs/faq.html), or [Guides](https://www.consul.io/docs/guides/index.html).

* Make sure you test against the latest released version. It is possible we already fixed the bug you're experiencing. However, if you are on an older version of Consul and feel the issue is critical, do let us know.

* Check existing issues (both open and closed) to make sure it has not been reported previously.

* Provide a reproducible test case. If a contributor can't reproduce an issue, then it dramatically lowers the chances it'll get fixed. And in some cases, the issue will eventually be closed.

* Aim to respond promptly to any questions made by the Consul team to your issue. Stale issues will be closed.

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
installed (version 1.10+ is _required_). Make sure you have Go properly installed,
including setting up your [GOPATH](https://golang.org/doc/code.html#GOPATH).

Next, clone this repository into `$GOPATH/src/github.com/hashicorp/consul` and then type `make dev`. 
In a few moments, you'll have a working `consul` executable in `consul/bin` and `$GOPATH/bin`:

>Note: `make dev` will build for your local machine's os/architecture. If you wish to build for all os/architecture combinations use `make`.

## Making Changes to Consul

The first step to making changes is to fork Consul. 
Afterwards, the easiest way to work on the fork is to set it as a remote of the Consul project:

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

>Note: If you make any changes to the code, run `make format` to automatically format the code according to Go standards.

## Testing

You can run tests locally by typing `make test`. The test suite may fail if over-parallelized, 
so if you are seeing stochastic failures try `GOTEST_FLAGS="-p 2 -parallel 2" make test`.

## Vendoring

Consul currently uses [govendor](https://github.com/kardianos/govendor) for
vendoring and [vendorfmt](https://github.com/magiconair/vendorfmt) for formatting
`vendor.json` to a more merge-friendly "one line per package" format.

If you are submitting a change that requires new or updated dependencies, 
please include them in `vendor/vendor.json` and in the `vendor/` folder. 
This helps everything get tested properly in CI.

Use `govendor fetch <project>` to add a project as a dependency. See govendor's [Quick Start](https://github.com/kardianos/govendor#quick-start-also-see-the-faq) for examples.

Please only apply the minimal vendor changes to get your PR to work. 
Consul does not attempt to track the latest version for each dependency.

