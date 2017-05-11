FROM ubuntu

RUN echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list
RUN apt-get update && apt-get upgrade -y
RUN apt-get install -y unzip wget

RUN wget https://dl.bintray.com/mitchellh/consul/0.1.0_linux_amd64.zip -O /tmp/consul.zip
RUN unzip /tmp/consul.zip && mv /tmp/consul /usr/bin
RUN sudo chmod +x /usr/bin/consul

EXPOSE 8300 8301 8302 8400 8500 8600
