# Contributing

If you submit a pull request, please keep the following guidelines in mind:

1. Code should be `go fmt` compliant.
2. Types, structs and funcs should be documented.
3. Tests pass.

## Getting set up

Assuming your `$GOPATH` is set up according to your desires, run:

```sh
go get github.com/digitalocean/godo
go get -u github.com/stretchr/testify/assert
```

## Running tests

When working on code in this repository, tests can be run via:

```sh
go test .
```

## Versioning

Godo follows [semver](https://www.semver.org) versioning semantics.  New functionality should be accompanied by increment to the minor version number. The current strategy is to release often. Any code which is complete, tested, reviewed, and merged to master is worthy of release.

## Releasing

Releasing a new version of godo is currently a manual process.

1. Update the `CHANGELOG.md` with your changes. If a version header for the next (unreleased) version does not exist, create one.  Include one bullet point for each piece of new functionality in the release, including the pull request ID, description, and author(s).

```
## [v1.8.0] - 2019-03-13

- #210 Expose tags on storage volume create/list/get. - @jcodybaker
- #123 Update test dependencies - @digitalocean
```

2. Update the `libraryVersion` number in `godo.go`.
3. Make a pull request with these changes.  This PR should be separate from the PR containing the godo changes.
4. Once the pull request has been merged, visit [https://github.com/digitalocean/godo/releases](https://github.com/digitalocean/godo/release) and click `Draft a new release`.  
5. Update the `Tag version` and `Release title` field with the new godo version.  Be sure the version has a `v` prefixed in both places. Ex `v1.8.0`.  
6. Copy the changelog bullet points to the description field.
7. Publish the release.