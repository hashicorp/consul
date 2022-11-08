FROM amazonlinux:2 as k6bin
ARG K6_VERSION=v0.39.0

RUN yum update -y \
 && yum install -y curl tar gzip zip

RUN curl -O -L "https://github.com/grafana/k6/releases/download/${K6_VERSION}/k6-${K6_VERSION}-linux-amd64.tar.gz"  \
 && tar -xvf k6-${K6_VERSION}-linux-amd64.tar.gz \
 && mv /k6-${K6_VERSION}-linux-amd64/k6 /usr/local/bin/


FROM public.ecr.aws/lambda/nodejs:14

COPY --from=k6bin /usr/local/bin/k6 /usr/local/bin/
COPY index.js loadtest.js service.json ${LAMBDA_TASK_ROOT}

WORKDIR /var/task
CMD ["index.handler"]
