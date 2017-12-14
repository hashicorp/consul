# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Apache Thrift Docker build environment for CentOS
#
# Known missing client libraries:
#  - dotnet (will update to 2.0.0 separately)
#  - haxe (not in centos)

FROM centos:7.3.1611
MAINTAINER Apache Thrift <dev@thrift.apache.org>

RUN yum install -y epel-release

# General dependencies
RUN yum install -y \
      autoconf \
      bison \
      bison-devel \
      clang \
      clang-analyzer \
      cmake3 \
      curl \
      flex \
      gcc \
      gcc-c++ \
      gdb \
      git \
      libtool \
      m4 \
      make \
      tar \
      unzip \
      valgrind \
      wget && \
      ln -s /usr/bin/cmake3 /usr/bin/cmake && \
      ln -s /usr/bin/cpack3 /usr/bin/cpack && \
      ln -s /usr/bin/ctest3 /usr/bin/ctest

# C++ dependencies
RUN yum install -y \
      boost-devel-static \
      zlib-devel \
      openssl-devel \
      libevent-devel && \
    cd /usr/lib64 && \
    ln -s libboost_thread-mt.a libboost_thread.a

# C# Dependencies
RUN yum install -y \
      mono-core \
      mono-devel \
      mono-web-devel \
      mono-extras

# D Dependencies
RUN yum install -y http://downloads.dlang.org/releases/2.x/2.076.0/dmd-2.076.0-0.fedora.x86_64.rpm xdg-utils
RUN curl -sSL https://github.com/D-Programming-Deimos/openssl/archive/master.tar.gz| tar xz && \
    curl -sSL https://github.com/D-Programming-Deimos/libevent/archive/master.tar.gz| tar xz && \
    mkdir -p /usr/include/dmd/druntime/import/deimos /usr/include/dmd/druntime/import/C && \
    mv libevent-master/deimos/* openssl-master/deimos/* /usr/include/dmd/druntime/import/deimos/ && \
    mv libevent-master/C/* openssl-master/C/* /usr/include/dmd/druntime/import/C/ && \
    rm -rf libevent-master openssl-master

# Dart
RUN cd /usr/local && \
    wget -q https://storage.googleapis.com/dart-archive/channels/stable/release/1.24.2/sdk/dartsdk-linux-x64-release.zip && \
    unzip -q dartsdk-linux-x64-release.zip && \
    rm dartsdk-linux-x64-release.zip
ENV PATH /usr/local/dart-sdk/bin:$PATH

# Erlang Dependencies
RUN curl -sSL http://packages.erlang-solutions.com/rpm/centos/erlang_solutions.repo -o /etc/yum.repos.d/erlang_solutions.repo && \
    yum install -y \
      erlang-kernel \
      erlang-erts \
      erlang-stdlib \
      erlang-eunit \
      erlang-rebar \
      erlang-tools

# GLibC Dependencies
RUN yum install -y glib2-devel

# Go Dependencies
RUN curl -sSL https://storage.googleapis.com/golang/go1.9.linux-amd64.tar.gz | tar -C /usr/local/ -xz
ENV PATH /usr/local/go/bin:$PATH

# Haskell Dependencies
RUN yum -y install haskell-platform

# Haxe Dependencies
# Not in debian/stretch

# Java Dependencies
RUN yum install -y \
      ant \
      junit \
      ant-junit \
      java-1.8.0-openjdk-devel

# Lua Dependencies
# Lua in epel is too old (5.1.4, need 5.2) so we get the latest
RUN yum install -y readline-devel && \
    wget -q http://www.lua.org/ftp/lua-5.3.4.tar.gz && \
    tar xzf lua-5.3.4.tar.gz && \
    cd lua-5.3.4 && \
    sed -i 's/CFLAGS= /CFLAGS= -fPIC /g' src/Makefile && \
    make linux && \
    make install && \
    cd .. && \
    rm -rf lua-5*

# MinGW Dependencies
RUN yum install -y \
      mingw32-binutils \
      mingw32-crt \
      mingw32-nsis

# Node.js Dependencies
# Work around epel issue where they removed http-parser that nodejs depends on!
RUN yum -y install https://opensource.enda.eu/packages/http-parser-2.7.1-3.el7.x86_64.rpm
RUN yum install -y \
      nodejs \
      npm

# Ocaml Dependencies
RUN yum install -y \
      ocaml \
      ocaml-ocamldoc && \
    wget -q https://raw.github.com/ocaml/opam/master/shell/opam_installer.sh -O - | sh -s /usr/local/bin && \
    opam init --yes && \
    opam install --yes oasis && \
    echo '. /root/.opam/opam-init/init.sh > /dev/null 2> /dev/null || true' >> ~/.bashrc

# Perl Dependencies
RUN yum install -y \
      perl \
      perl-version \
      perl-Bit-Vector \
      perl-Class-Accessor \
      perl-ExtUtils-MakeMaker \
      perl-Test-Simple \
      perl-IO-Socket-SSL \
      perl-Net-SSLeay \
      perl-Crypt-SSLeay

# PHP Dependencies
RUN yum install -y \
      php \
      php-devel \
      php-pear \
      re2c \
      php-phpunit-PHPUnit \
      bzip2

# Python Dependencies
RUN yum install -y \
      python \
      python-devel \
      python-pip \
      python-setuptools \
      python34 \
      python34-devel \
      python34-pip \
      python34-setuptools
RUN pip2 install --upgrade pip
RUN pip2 install --upgrade backports.ssl_match_hostname ipaddress setuptools six tornado tornado-testing twisted virtualenv zope-interface
RUN pip3 install --upgrade pip
RUN pip3 install --upgrade backports.ssl_match_hostname ipaddress setuptools six tornado tornado-testing twisted virtualenv zope-interface

# Ruby Dependencies
RUN yum install -y \
      ruby \
      ruby-devel \
      rubygems && \
    gem install bundler rake

# Rust
RUN curl https://sh.rustup.rs -sSf | sh -s -- -y --default-toolchain 1.17.0
ENV PATH /root/.cargo/bin:$PATH

# Clean up
RUN rm -rf /tmp/* && \
    yum clean all

ENV THRIFT_ROOT /thrift
RUN mkdir -p $THRIFT_ROOT/src
COPY Dockerfile $THRIFT_ROOT/
WORKDIR $THRIFT_ROOT/src
