ARG VERSION=7
ARG IMAGE=public.ecr.aws/datadog/agent
FROM $IMAGE:$VERSION

COPY datadog.yaml /etc/datadog-agent/datadog.yaml
COPY conf.yaml /etc/datadog-agent/conf.d/consul.d/conf.yaml
