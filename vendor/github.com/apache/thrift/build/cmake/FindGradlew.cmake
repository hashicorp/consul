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


#  GRADLEW_FOUND - system has Gradlew
#  GRADLEW_EXECUTABLE - the Gradlew executable
#
# It will search the location CMAKE_SOURCE_DIR/lib/java

include(FindPackageHandleStandardArgs)

find_program(GRADLEW_EXECUTABLE gradlew PATHS ${CMAKE_SOURCE_DIR}/lib/java NO_DEFAULT_PATH NO_CMAKE_FIND_ROOT_PATH)
find_package_handle_standard_args(Gradlew DEFAULT_MSG GRADLEW_EXECUTABLE)
mark_as_advanced(GRADLEW_EXECUTABLE)

# Buggy find_program cannot find gradlew.bat when gradlew is at the same path
# and even buggier ctest will not execute gradlew.bat when gradlew is given.
if(CMAKE_HOST_WIN32)
    string(REGEX REPLACE "(.+gradlew)$" "\\1.bat" GRADLEW_EXECUTABLE ${GRADLEW_EXECUTABLE})
endif(CMAKE_HOST_WIN32)
