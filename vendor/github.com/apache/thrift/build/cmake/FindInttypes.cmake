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

# find msinttypes on compilers that don't provide it, for example
#   VS2010

# Usage: 
# Provide INTTYPES_ROOT if you need it
# Result: INTTYPES_INCLUDE_DIRS, where to find inttypes.h
# Result: Inttypes_FOUND, If false, inttypes.h was not found

find_path(INTTYPES_INCLUDE_DIRS inttypes.h HINTS ${INTTYPES_ROOT})
if (INTTYPES_INCLUDE_DIRS)
  set(Inttypes_FOUND TRUE)
else ()
  set(Inttypes_FOUND FALSE)
  if (Inttypes_FIND_REQUIRED)
    message(FATAL_ERROR "Could NOT find inttypes.h")
  endif ()
  message(STATUS "inttypes.h NOT found")
endif ()

mark_as_advanced(
  INTTYPES_INCLUDE_DIRS
)
