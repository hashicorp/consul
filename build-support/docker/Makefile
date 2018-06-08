ifeq ($(FORCE_REBUILD),1)
NOCACHE=--no-cache
else
NOCACHE=
endif
GO_BUILD_TAG?=consul-build-go
UI_BUILD_TAG?=consul-build-ui
UI_LEGACY_BUILD_TAG?=consul-build-ui-legacy

images: go-build-image ui-build-image ui-legacy-build-image

go-build-image:
	docker build $(NOCACHE) -t $(GO_BUILD_TAG) -f Build-Go.dockerfile .
	
ui-build-image:
	docker build $(NOCACHE) -t $(UI_BUILD_TAG) -f Build-UI.dockerfile .
	
ui-legacy-build-image:
	docker build $(NOCACHE) -t $(UI_LEGACY_BUILD_TAG) -f Build-UI-Legacy.dockerfile .
	
.PHONY: images go-build-image ui-build-image ui-legacy-build-image
