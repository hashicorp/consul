.PHONY: clean dist dist-docker dist-vercel test-workspace

# make is run with no target from the consul root Makefile ui target but from
# within docker during release, so that we keep that as the default target
.DEFAULT_GOAL := dist-docker

# Run from CI, should run any checks/tests for the workspace that we'd like to
# be run for every PR
test-workspace:
	yarn run compliance

# Called from the build ui docker image
dist-docker: dist

clean:
	rm -rf ./dist

# Build a distribution of the UI
dist: clean
	cd packages/consul-ui && \
		$(MAKE)

# Install deps for the UI only
deps: clean
	cd packages/consul-ui && \
		$(MAKE) deps

# Build a distribution of the UI for Vercel previews.
# The distribution must be copied into the ui/ subfolder
# in order to mirror the go binary
#
# Vercel Settings:
# ---
# Base Directory: ui
# Build Command: make dist-vercel
# Publish Directory: ui/dist
dist-vercel: clean
	mkdir -p dist/ui && \
	cd packages/consul-ui && \
		$(MAKE) build-staging && \
		mv dist/* ../../dist/ui
