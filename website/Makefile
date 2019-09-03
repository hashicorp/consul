VERSION?="0.3.41"

# The volume mounting steps are a way to exclude all the consul docs from being built
# in the Consul site build process while still including the index.html page that points
# users to learn.hashicorp.com. See PR#5847.

build:
	@echo "==> Starting build in Docker..."
	@docker run \
		--interactive \
		--rm \
		--tty \
		--volume "$(shell pwd):/website" \
		--volume "/website/source/docs/guides" \
		--volume "$(shell pwd)/source/docs/guides/index.html.md:/website/source/docs/guides/index.html.md" \
		-e "ENV=production" \
		hashicorp/middleman-hashicorp:${VERSION} \
		bundle exec middleman build --verbose --clean

website:
	@echo "==> Starting website in Docker..."
	@docker run \
		--interactive \
		--rm \
		--tty \
		--publish "4567:4567" \
		--publish "35729:35729" \
		--volume "$(shell pwd):/website" \
		--volume "/website/source/docs/guides" \
		--volume "$(shell pwd)/source/docs/guides/index.html.md:/website/source/docs/guides/index.html.md" \
		hashicorp/middleman-hashicorp:${VERSION}

.PHONY: build website
