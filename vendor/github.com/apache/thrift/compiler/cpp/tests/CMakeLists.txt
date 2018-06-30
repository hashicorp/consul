#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements. See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership. The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
#
cmake_minimum_required(VERSION 2.8.12)

project(thrift_compiler_tests)

set(THRIFT_COMPILER_SOURCE_DIR
    ${CMAKE_CURRENT_SOURCE_DIR}/..
)

# don't generate ZERO_CHECK
set(CMAKE_SUPPRESS_REGENERATION true)

configure_file(${THRIFT_COMPILER_SOURCE_DIR}/src/thrift/version.h.in ${CMAKE_CURRENT_BINARY_DIR}/thrift/version.h)
if(MSVC)
    # The winflexbison generator outputs some macros that conflict with the Visual Studio 2010 copy of stdint.h
    # This might be fixed in later versions of Visual Studio, but an easy solution is to include stdint.h first
    if(HAVE_STDINT_H)
        add_definitions(-D__STDC_LIMIT_MACROS)
        add_definitions(/FI"stdint.h")
    endif(HAVE_STDINT_H)
endif()

find_package(FLEX REQUIRED)
find_package(BISON REQUIRED)

# create directory for thrifty and thriftl
file(MAKE_DIRECTORY ${CMAKE_CURRENT_BINARY_DIR}/thrift/)

# Create flex and bison files and build the lib parse static library
BISON_TARGET(thrifty ${THRIFT_COMPILER_SOURCE_DIR}/src/thrift/thrifty.yy ${CMAKE_CURRENT_BINARY_DIR}/thrift/thrifty.cc)
FLEX_TARGET(thriftl ${THRIFT_COMPILER_SOURCE_DIR}/src/thrift/thriftl.ll ${CMAKE_CURRENT_BINARY_DIR}/thrift/thriftl.cc)
ADD_FLEX_BISON_DEPENDENCY(thriftl thrifty)

set(parse_SOURCES
    ${CMAKE_CURRENT_BINARY_DIR}/thrift/thrifty.cc
    ${CMAKE_CURRENT_BINARY_DIR}/thrift/thriftl.cc
    ${CMAKE_CURRENT_BINARY_DIR}/thrift/thrifty.hh
)

add_library(parse STATIC ${parse_SOURCES})

# Thrift compiler tests
set(thrift_compiler_tests
)

# you can add some files manually there 
set(thrift_compiler_tests_manual_SOURCES
    # tests file to avoid main in every test file
    ${CMAKE_CURRENT_SOURCE_DIR}/tests_main.cc
)

# set variable for tests sources - will be filled later
set(thrift_compiler_tests_SOURCES
)

set(thrift_compiler_SOURCES
    ${THRIFT_COMPILER_SOURCE_DIR}/src/thrift/logging.cc # we use logging instead of main to avoid breaking compillation (2 main v)
    ${THRIFT_COMPILER_SOURCE_DIR}/src/thrift/audit/t_audit.cpp
    ${THRIFT_COMPILER_SOURCE_DIR}/src/thrift/common.cc
    ${THRIFT_COMPILER_SOURCE_DIR}/src/thrift/generate/t_generator.cc
    ${THRIFT_COMPILER_SOURCE_DIR}/src/thrift/parse/t_typedef.cc
    ${THRIFT_COMPILER_SOURCE_DIR}/src/thrift/parse/parse.cc
    ${CMAKE_CURRENT_BINARY_DIR}/thrift/version.h
)

# This macro adds an option THRIFT_COMPILER_${NAME}
# that allows enabling or disabling certain languages
macro(THRIFT_ADD_COMPILER name description initial)
    string(TOUPPER "THRIFT_COMPILER_${name}" enabler)
    set(src "${THRIFT_COMPILER_SOURCE_DIR}/src/thrift/generate/t_${name}_generator.cc")
    option(${enabler} ${description} ${initial})
    if(${enabler})
        list(APPEND thrift_compiler_SOURCES ${src})
        file(GLOB thrift_compiler_tests_SOURCES
            "${CMAKE_CURRENT_SOURCE_DIR}/${name}/*.c*"
            "${CMAKE_CURRENT_SOURCE_DIR}/${name}/*.thrift"
        )
    endif()
endmacro()

# The following compiler with unit tests can be enabled or disabled
THRIFT_ADD_COMPILER(c_glib  "Enable compiler for C with Glib" OFF)
THRIFT_ADD_COMPILER(cpp     "Enable compiler for C++" OFF)
THRIFT_ADD_COMPILER(java    "Enable compiler for Java"   OFF)
THRIFT_ADD_COMPILER(as3     "Enable compiler for ActionScript 3" OFF)
THRIFT_ADD_COMPILER(dart    "Enable compiler for Dart" OFF)
THRIFT_ADD_COMPILER(haxe    "Enable compiler for Haxe" OFF)
THRIFT_ADD_COMPILER(csharp  "Enable compiler for C#" OFF)
THRIFT_ADD_COMPILER(netcore "Enable compiler for .NET Core" ON)
THRIFT_ADD_COMPILER(py      "Enable compiler for Python 2.0" OFF)
THRIFT_ADD_COMPILER(rb      "Enable compiler for Ruby" OFF)
THRIFT_ADD_COMPILER(perl    "Enable compiler for Perl" OFF)
THRIFT_ADD_COMPILER(php     "Enable compiler for PHP" OFF)
THRIFT_ADD_COMPILER(erl     "Enable compiler for Erlang" OFF)
THRIFT_ADD_COMPILER(cocoa   "Enable compiler for Cocoa Objective-C" OFF)
THRIFT_ADD_COMPILER(swift   "Enable compiler for Cocoa Swift" OFF)
THRIFT_ADD_COMPILER(st      "Enable compiler for Smalltalk" OFF)
THRIFT_ADD_COMPILER(ocaml   "Enable compiler for OCaml" OFF)
THRIFT_ADD_COMPILER(hs      "Enable compiler for Haskell" OFF)
THRIFT_ADD_COMPILER(xsd     "Enable compiler for XSD" OFF)
THRIFT_ADD_COMPILER(html    "Enable compiler for HTML Documentation" OFF)
THRIFT_ADD_COMPILER(js      "Enable compiler for JavaScript" OFF)
THRIFT_ADD_COMPILER(json    "Enable compiler for JSON" OFF)
THRIFT_ADD_COMPILER(javame  "Enable compiler for Java ME" OFF)
THRIFT_ADD_COMPILER(delphi  "Enable compiler for Delphi" OFF)
THRIFT_ADD_COMPILER(go      "Enable compiler for Go" OFF)
THRIFT_ADD_COMPILER(d       "Enable compiler for D" OFF)
THRIFT_ADD_COMPILER(lua     "Enable compiler for Lua" OFF)
THRIFT_ADD_COMPILER(gv      "Enable compiler for GraphViz" OFF)
THRIFT_ADD_COMPILER(rs      "Enable compiler for Rust" OFF)
THRIFT_ADD_COMPILER(xml     "Enable compiler for XML" OFF)

# Thrift is looking for include files in the src directory
# we also add the current binary directory for generated files
include_directories(${CMAKE_CURRENT_BINARY_DIR} ${THRIFT_COMPILER_SOURCE_DIR}/src ${CMAKE_CURRENT_SOURCE_DIR}/catch)

add_library(thrift_compiler ${thrift_compiler_SOURCES})

#link parse lib to thrift_compiler lib
target_link_libraries(thrift_compiler parse)

# add tests executable
add_executable(thrift_compiler_tests ${thrift_compiler_tests_manual_SOURCES} ${thrift_compiler_tests_SOURCES})

# if generates for Visual Studio set thrift_compiler_tests as default project
if(MSVC)
    set_property(TARGET thrift_compiler_tests PROPERTY VS_STARTUP_PROJECT thrift_compiler_tests)
endif()

set_target_properties(thrift_compiler_tests PROPERTIES RUNTIME_OUTPUT_DIRECTORY bin/)
set_target_properties(thrift_compiler_tests PROPERTIES OUTPUT_NAME thrift_compiler_tests)

target_link_libraries(thrift_compiler_tests thrift_compiler)

enable_testing()
add_test(NAME ThriftTests COMMAND thrift_compiler_tests)