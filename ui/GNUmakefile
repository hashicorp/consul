.PHONY: clean dist dist-docker dist-netlify
# Called from the build ui docker image
dist-docker: dist

clean:
	rm -rf ./dist

# Build a distribution of the UI using the minimal amount of dependencies
dist: clean
	cd packages/consul-ui && \
		CONSUL_UI_INSTALL_FLAGS=--focus \
		$(MAKE)

# Build a distribution of the UI for Netlify previews
#
# Netlify Settings:
# ---
# base-directory: ui
# build command: make dist-netlify
# publish directory: ui/dist

dist-netlify: clean
	mkdir -p dist && \
	cd packages/consul-ui && \
		CONSUL_UI_INSTALL_FLAGS=--focus \
		$(MAKE) build-staging && \
		mv dist/* ../../dist && \
		cp _redirects ../../dist/_redirects


