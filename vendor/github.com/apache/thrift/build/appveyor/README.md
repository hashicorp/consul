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

# Appveyor Build

Appveyor is capable of building MSVC 2010 through 2015 as well as
having the latest MSYS2/MinGW 64-bit environment.  It has many versions
of boost and python installed as well.  See what appveyor has
[installed on build workers](https://www.appveyor.com/docs/installed-software/).

We run a matrix build on Appveyor and build the following combinations:

* MinGW x64 (gcc 6.3.0)
* MSVC 2010 x86, an older boost, an older python
* MSVC 2015 x86/x64, the latest boost, the latest python
* MSYS2 x64 (gcc 6.3.0) - this is a work in progress

The Appveyor script takes the first four letters from the PROFILE specified in
the environment stanza and runs these scripts in order:

????-appveyor-install.bat will install third party libraries and set up the environment
????-appveyor-build.bat will build with cmake
????-appveyor-test.bat will run ctest
