<!---
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->

# Building thrift on Windows (MinGW64/MSYS2)

Thrift uses cmake to make it easier to build the project on multiple platforms, however to build a fully functional and production ready thrift on Windows requires a number of third party libraries to be obtained.  Once third party libraries are ready, the right combination of options must be passed to cmake in order to generate the correct environment.

> Note: libevent and libevent-devel do not work with this toolchain as they do not properly detect mingw64 and expect some headers to exist that do not, so the non-blocking server is not currently built into this solution.

## MSYS2

Download and fully upgrade msys2 following the instructions at:

    https://msys2.github.io/

Install the necessary toolchain items for C++:

    $ pacman --needed -S bison flex make mingw-w64-x86_64-openssl \
                mingw-w64-x86_64-boost mingw-w64-x86_64-cmake \
                mingw-w64-x86_64-toolchain mingw-w64-x86_64-zlib

Update your msys2 bash path to include /mingw64/bin by adding a line to your ~/.bash_profiles using this command:

    echo "export PATH=/mingw64/bin:\$PATH" >> ~/.bash_profile

After that, close your shell and open a new one.

Use cmake to create a MinGW makefile, out of tree (assumes you are in the top level of the thrift source tree):

    mkdir ../thrift-build
    cd ../thrift-build
    cmake -G"MinGW Makefiles" -DCMAKE_MAKE_PROGRAM=/mingw64/bin/mingw32-make \
       -DCMAKE_C_COMPILER=x86_64-w64-mingw32-gcc.exe \
       -DCMAKE_CXX_COMPILER=x86_64-w64-mingw32-g++.exe \
       -DWITH_BOOSTTHREADS=ON -DWITH_LIBEVENT=OFF \
       -DWITH_SHARED_LIB=OFF -DWITH_STATIC_LIB=ON \
       -DWITH_JAVA=OFF -DWITH_PYTHON=OFF -DWITH_PERL=OFF \
       ../thrift

Build thrift (inside thrift-build):

    cmake --build .

Run the tests (inside thrift-build):

    ctest

> If you run into issues, check Apache Jira THRIFT-4046 for patches relating to MinGW64/MSYS2 builds.

## Tested With

msys2 64-bit 2016-10-26 distribution
