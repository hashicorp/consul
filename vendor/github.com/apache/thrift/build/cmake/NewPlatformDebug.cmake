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

#
# For debugging new platforms, just to see what some environment flags are...
#
macro(SHOWFLAG flag)
  message(STATUS "${flag} = ${${flag}}")
endmacro(SHOWFLAG)

set(NEWPLATFORMDEBUG ON)

if(NEWPLATFORMDEBUG)
  SHOWFLAG("APPLE")
  SHOWFLAG("BORLAND")
  SHOWFLAG("CMAKE_C_COMPILER_ID")
  SHOWFLAG("CMAKE_CXX_COMPILER_ID")
  SHOWFLAG("CMAKE_COMPILER_IS_GNUCC")
  SHOWFLAG("CMAKE_COMPILER_IS_GNUCXX")
  SHOWFLAG("CYGWIN")
  SHOWFLAG("MINGW")
  SHOWFLAG("MSVC")
  SHOWFLAG("MSVC_VERSION")
  SHOWFLAG("MSYS")
  SHOWFLAG("UNIX")
  SHOWFLAG("WATCOM")
  SHOWFLAG("WIN32")
endif(NEWPLATFORMDEBUG)
