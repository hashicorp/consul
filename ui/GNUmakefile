# Called from the build ui docker image
dist-docker: dist

# Build a distribution of the UI using the minimal amount of dependencies
dist:
	cd packages/consul-ui && \
		CONSUL_UI_INSTALL_FLAGS=--focus \
		$(MAKE)
