# Note this is the same as the final stage of Consul-Dev-CI.dockerfile and
# should be kept roughly in sync.
FROM consul:latest

# Consul binary from previous build job gets mounted into the CWD for the docker
# build. Copy it to final location.
ADD ./consul /bin
