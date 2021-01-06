.PHONY: clean dist dist-docker dist-netlify test-workspace

# Run from CI, should run any checks/tests for the workspace that we'd like to
# be run for every PR
test-workspace:
	yarn run compliance

# Called from the build ui docker image
dist-docker: dist

clean:
	rm -rf ./dist

# Build a distribution of the UI using the minimal amount of dependencies
dist: clean
	cd packages/consul-ui && \
		CONSUL_UI_INSTALL_FLAGS=--focus \
		$(MAKE)

# Build a distribution of the UI for Netlify previews.
# The distribution must be copied into the ui/ subfolder
# in order to mirror the go binary
#
# Netlify Settings:
# ---
# base-directory: ui
# build command: make dist-netlify
# publish directory: ui/dist
dist-netlify: clean
	mkdir -p dist/ui && \
	cd packages/consul-ui && \
		CONSUL_UI_INSTALL_FLAGS=--focus \
		$(MAKE) build-staging && \
		mv dist/* ../../dist/ui && \
		cp _redirects ../../dist/_redirects
