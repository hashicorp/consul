FROM ubuntu:14.04
MAINTAINER Steffen Roegner 'steffen.roegner@gmail.com'

ENV DEBIAN_FRONTEND noninteractive
ENV REFRESHED_AT 2014-10-17

RUN apt-get update
RUN apt-get -y install curl unzip

ENV CONSUL_VERSION 0.4.1

RUN mkdir -p /etc/consul /var/lib/consul/data /var/lib/consul/ui /var/lib/consul/scripts; \
    curl -L https://dl.bintray.com/mitchellh/consul/${CONSUL_VERSION}_linux_amd64.zip -o /tmp/c.zip && \
    curl -L https://dl.bintray.com/mitchellh/consul/${CONSUL_VERSION}_web_ui.zip -o /tmp/ui.zip && \
    unzip /tmp/c.zip -d /usr/sbin; \
    unzip /tmp/ui.zip -d /var/lib/consul/ui

VOLUME /etc/consul

EXPOSE 8400 8500 8600 53

ENTRYPOINT ["/usr/sbin/consul", "agent", "-config-dir=/etc/consul"]
