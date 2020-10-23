# release helpers
.PHONY: major minor patch release verify
NPM=npm
major:
minor:
patch:
	@VERSION=$$($(NPM) ls | head -n1 | awk -F'[@ ]' '{ print $$3}') \
		TAG=$$($(NPM) version --no-git-tag-version $@ | sed 's/v//g') \
			&& git commit -i package*json -m "Bump from $$VERSION to $$TAG" \
			&& git tag $$TAG
verify:
	@PACKAGE=$$($(NPM) ls | head -n1 | awk '{ print $$1}') \
		VERSION=$$(echo $$PACKAGE | awk -F'[@ ]' '{ print $$3 }') \
		TAG=$$(git describe --tags --dirty 2> /dev/null || echo '-') \
		&& if [ "$$VERSION" = "$$TAG" ]; \
				then echo "Able to release $$PACKAGE"; \
				else echo "Unable to release (package: $$VERSION != git: $$TAG)" && exit 1; \
			fi \
		&& echo $$PACKAGE \
		&& $(NPM) pack --dry-run
release: verify;
	@echo "Releasing as '$$($(NPM) whoami)'" \
	  && git push && git push --tags
	#&& npm publish . --access public


