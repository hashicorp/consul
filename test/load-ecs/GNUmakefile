SHELL = bash

TF_VAR_aws_region ?= "us-east-1"
AWS_ACCOUNT_ID=$(shell aws sts get-caller-identity | jq -r .Account)
CLUSTER_NAME ?= consul-example

# Refresh certificates, build and deploy images
.PHONY: all
all: certs images

# Build CA & Certs for Consul
.PHONY: certs
certs: clean-certs
	cd tls && consul tls ca create && \
    for I in "-server" "-client" "-cli"; do \
      consul tls cert create $$I -dc dc1; \
    done; \
    cd ..

# Clean out local CA & Cert
.PHONY: clean-certs
clean-certs:
	rm -rf tls && mkdir tls && touch tls/.gitkeep

# Make server container
.PHONY: server
server:
	docker build --platform linux/amd64 --tag consul-server:local --target consul-server ./containers/consul

# Make agent container
.PHONY: agent
agent:
	docker build --platform linux/amd64 --tag consul-agent:local --target consul-agent ./containers/consul

# Make datadog container
.PHONY: datadog
datadog:
	docker build --platform linux/amd64 --tag datadog-agent:local ./containers/datadog

# Make k6 container
.PHONY: k6
k6:
	docker build --platform linux/amd64 --tag k6:local ./containers/k6

# Build all the test images
.PHONY: build
build: server agent datadog k6

# Push the test images to ECR
.PHONY: push
push:
	@test "$(TF_VAR_aws_region)" || $(shell echo "Failed to find TF_VAR_aws_region from environment" && exit 1)
	@test "$(AWS_ACCOUNT_ID)" || $(shell echo "Failed to find AWS_ACCOUNT_ID from caller identity" && exit 1)
	@test "$(CLUSTER_NAME)" || $(shell echo "Failed to find CLUSTER_NAME from caller identity" && exit 1)
	@aws ecr get-login-password --region $(TF_VAR_aws_region) | docker login --username AWS --password-stdin "$(AWS_ACCOUNT_ID).dkr.ecr.$(TF_VAR_aws_region).amazonaws.com"
	@docker tag "consul-server:local" "$(AWS_ACCOUNT_ID).dkr.ecr.$(TF_VAR_aws_region).amazonaws.com/$(CLUSTER_NAME)/consul:server"
	@docker tag "consul-agent:local" "$(AWS_ACCOUNT_ID).dkr.ecr.$(TF_VAR_aws_region).amazonaws.com/$(CLUSTER_NAME)/consul:agent"
	@docker tag "datadog-agent:local" "$(AWS_ACCOUNT_ID).dkr.ecr.$(TF_VAR_aws_region).amazonaws.com/$(CLUSTER_NAME)/consul:datadog"
	@docker tag "k6:local" "$(AWS_ACCOUNT_ID).dkr.ecr.$(TF_VAR_aws_region).amazonaws.com/$(CLUSTER_NAME)/consul:k6"
	docker push "$(AWS_ACCOUNT_ID).dkr.ecr.$(TF_VAR_aws_region).amazonaws.com/$(CLUSTER_NAME)/consul:server"
	docker push "$(AWS_ACCOUNT_ID).dkr.ecr.$(TF_VAR_aws_region).amazonaws.com/$(CLUSTER_NAME)/consul:agent"
	docker push "$(AWS_ACCOUNT_ID).dkr.ecr.$(TF_VAR_aws_region).amazonaws.com/$(CLUSTER_NAME)/consul:datadog"
	docker push "$(AWS_ACCOUNT_ID).dkr.ecr.$(TF_VAR_aws_region).amazonaws.com/$(CLUSTER_NAME)/consul:k6"


# Pre-build some infra, so that ECR can exist before docker build tag push
.PHONY: repos
repos:
	terraform apply -auto-approve

# Builds and Push Images to ECR repos
.PHONY: images
images: build push

# Deploy containers and lambdas that depend on ECR images
.PHONY: infra
infra:
	terraform apply -var 'deploy_consul_ecs=true' -var 'deploy_efs_cluster=true' -auto-approve

# Execute the K6 Load test ideally after the infra stage has completed and has had time to start the containers
.PHONY: test-ecs
test-ecs:
	terraform apply -var 'deploy_consul_ecs=true' -var 'deploy_efs_cluster=true' -var 'run_k6=true' -auto-approve

# Clean the environment
.PHONY: clean
clean:
	terraform destroy -auto-approve
