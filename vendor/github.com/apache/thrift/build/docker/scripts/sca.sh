#!/bin/bash
set -ev

#
# Generate thrift files so the static code analysis includes an analysis
# of the files the thrift compiler spits out.  If running interactively
# set the NOBUILD environment variable to skip the boot/config/make phase.
#

if [[ -z "$NOBUILD" ]]; then
  ./bootstrap.sh
  ./configure --enable-tutorial=no
  make -j3 precross
fi

#
# C/C++ static code analysis with cppcheck
# add --error-exitcode=1 to --enable=all as soon as everything is fixed
#
# Python code style check with flake8
#
# search for TODO etc within source tree
# some statistics about the code base
# some info about the build machine

# Compiler cppcheck (All)
cppcheck --force --quiet --inline-suppr --enable=all -j2 compiler/cpp/src

# C++ cppcheck (All)
cppcheck --force --quiet --inline-suppr --enable=all -j2 lib/cpp/src lib/cpp/test test/cpp tutorial/cpp

# C Glib cppcheck (All)
cppcheck --force --quiet --inline-suppr --enable=all -j2 lib/c_glib/src lib/c_glib/test test/c_glib/src tutorial/c_glib

# Silent error checks
# See THRIFT-4371 : flex generated code triggers "possible null pointer dereference" in yy_init_buffer
cppcheck --force --quiet --inline-suppr --suppress="*:thrift/thriftl.cc" --error-exitcode=1 -j2 compiler/cpp/src
cppcheck --force --quiet --inline-suppr --error-exitcode=1 -j2 lib/cpp/src lib/cpp/test test/cpp tutorial/cpp
cppcheck --force --quiet --inline-suppr --error-exitcode=1 -j2 lib/c_glib/src lib/c_glib/test test/c_glib/src tutorial/c_glib

# Python code style
flake8 --ignore=E501 --exclude=lib/py/build lib/py
flake8 --exclude=tutorial/py/build tutorial/py
# THRIFT-4371 : generated files are excluded because they haven't been scrubbed yet
flake8 --ignore=E501 --exclude="*/gen-py*/*",test/py/build test/py
flake8 test/py.twisted
flake8 test/py.tornado
flake8 --ignore=E501 test/test.py
flake8 --ignore=E501,E722 test/crossrunner
flake8 test/features

# PHP code style
composer install --quiet
./vendor/bin/phpcs

# TODO etc
echo FIXMEs: `grep -r FIXME * | wc -l`
echo  HACKs: `grep -r HACK * | wc -l`
echo  TODOs: `grep -r TODO * | wc -l`

# LoC
sloccount .

# System Info
dpkg -l
uname -a
