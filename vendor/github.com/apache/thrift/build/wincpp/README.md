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

# Building thrift on Windows (Native)

Thrift uses cmake to make it easier to build the project on multiple platforms, however to build a fully functional and production ready thrift on Windows requires a number of third party libraries to be obtained or built.  Once third party libraries are ready, the right combination of options must be passed to cmake in order to generate the correct environment.

## Summary

These instructions will help you build thrift for windows using Visual
Studio 2010 or later.  The contributed batch files will help you build
the third party libraries needed for complete thrift functionality as
well as thrift itself.

These instructions follow a directory layout that looks like the following:

    workspace\
      build\       - this is where the out-of-tree thrift cmake builds are generated
      dist\        - this is where the thrift build results end up
      thirdparty\  - this is where all third party binaries and libraries live
        build\       - this is where all third party out-of-tree builds are generated
                       (except for openssl, which only builds in-tree)
        dist\        - this is where all third party distributions end up
        src\         - this is where all third party source projects live
      scripts\     - batch files used to set environment variables for builds
      thrift\      - this is where the thrift source project lives

Create a "workspace" directory somewhere on your system and then copy the contents of this
directory to there, then clone or unpack thrift into `workspace\thrift`.

## Third Party Libraries

Batch scripts are provided to build some third party libraries.  You must download them and place them into the directory noted for each.  You can use different versions if you prefer; these instructions were made with the versions listed.  

> TIP: To modify the versions used in the batch scripts, look in scripts\tpversions.bat.

Build them in the order listed to satisfy their dependencies.

### winflexbison

        source: web site
      location: https://sourceforge.net/projects/winflexbison/files/win_flex_bison-latest.zip/download
       version: "latest"
     directory: workspace\thirdparty\dist\winflexbison

This package is required to build the compiler.  This third party package does not need to be built as it is a binary distribution of the "bison" and "flex" tools normally found on Unix boxes.

> TIP: If you are only interested in building the compiler, you can skip the remaining third party libraries.

### zlib

        source: web site
      location: http://zlib.net/
       version: 1.2.9
     directory: workspace\thirdparty\src\zlib-1.2.9

To build, open the appropriate Visual Studio command prompt and then run 
the build-zlib.bat script in thirdparty\src.
 
### openssl

        source: web site
      location: https://www.openssl.org/
       version: 1.1.0c
     directory: workspace\thirdparty\src\openssl-1.1.0c
    depends-on: zlib

If you are using openssl-1.1.0 or later, they changed static builds to use Microsoft Static RTL for release builds.  zlib by default uses a dynamic runtime, as does libevent.  Edit the file Configurations/10-main.conf and replace the section contents for "VC-noCE-common" with what appears below to make openssl build with dynamic runtime instead:

    "VC-noCE-common" => {
        inherit_from     => [ "VC-common" ],
        template         => 1,
        cflags           => add(picker(default => "-DUNICODE -D_UNICODE",
                                       debug   => "/MDd /Od -DDEBUG -D_DEBUG",
                                       release => "/MD /O2"
                                      )),
        bin_cflags       => add(picker(debug   => "/MDd",
                                       release => "/MD",
                                      )),
        bin_lflags       => add("/subsystem:console /opt:ref"),
        ex_libs          => add(sub {
            my @ex_libs = ();
            push @ex_libs, 'ws2_32.lib' unless $disabled{sock};
            push @ex_libs, 'gdi32.lib advapi32.lib crypt32.lib user32.lib';
            return join(" ", @ex_libs);
        }),
    },

To build, open the appropriate Visual Studio command prompt and then run 
the build-openssl.bat script in thirdparty\src.

### libevent

        source: git
      location: https://github.com/nmathewson/Libevent.git
           use: commit 3821cca1a637f4da4099c9343e7326da00f6981c or later
          date: Fri Dec 23 16:19:35 2016 +0800 or later
       version: corresponds to 2.1.7rc + patches
     directory: workspace\thirdparty\src\libevent-2.1.7rc2
    depends-on: openssl, zlib

To build, open the appropriate Visual Studio command prompt and then run 
the build-libevent.bat script in thirdparty\src.

### msinttypes

        source: web site
      location: https://code.google.com/archive/p/msinttypes/downloads
       version: 26
     directory: workspace\thirdparty\dist\msinttypes

> TIP: This is only necessary for Visual Studio 2010, which did not include an <inttypes.h> header.

This third party package does not need to be built as it is a distribution of header files.

### boost

        source: web site
      location: http://boost.teeks99.com/
       version: 1_62_0
     directory: workspace\thirdparty\dist\boost_1_62_0

The pre-built binary versions of boost come in self-unpacking executables.  Run each of the ones you are interested in and point them at the same thirdparty dist directory.

## Building a Production thrift Compiler

### Prerequisites

* CMake-2.8.12.2 or later
* Visual Studio 2010 or later
* thrift source placed into workspace\thrift
* winflexbison placed into workspace\thirdparty\dist

### Instructions

By following these instructions you will end up with a release mode thrift compiler that is suitable for distribution as it has no external dependencies.  

1. Open the appropriate Visual Studio Command Prompt.
2. `cd workspace`
3. `build-thrift-compiler.bat`

The batch file uses CMake to generate an out-of-tree build directory in `workspace\build` and then builds the compiler.  The resulting `thrift.exe` program is placed into `workspace\dist` in a path that depends on your compiler version and platform.  For example, if you use a Visual Studio 2010 x64 Command Prompt, the compiler will be placed into `workspace\dist\thrift-compiler-dev\vc100\x64\Release\thrift.exe`

#### Details

This section is for those who are curious about the CMake options used in the build process.

CMake takes the source tree as the first argument and uses the remaining arguments for configuration.  The batch file `build-thrift-compiler` essentially performs the following commands:

    C:\> CD workspace\build
    C:\workspace\build> "C:\Program Files\CMake\bin\cmake.exe" ..\thrift 
                          -DBISON_EXECUTABLE=..\thirdparty\dist\winflexbison\win_bison.exe
                          -DCMAKE_BUILD_TYPE=Release
                          -DFLEX_EXECUTABLE=..\thirdparty\dist\winflexbison\win_flex.exe
                          -DWITH_MT=ON
                          -DWITH_SHARED_LIB=OFF
                          -G"NMake Makefiles"
    C:\workspace\build> NMAKE /FMakefile thrift-compiler

WITH_MT controls the dynamic or static runtime library selection.  To build a production compiler, the thrift project recommends using the static runtime library to make the executable portable.  The batch file sets this.

You can build a Visual Studio project file by following the example but substituting a different generator for the "-G" option.  Run `cmake.exe --help` for a list of generators.  Typically, this is one of the following on Windows (omit "Win64" to build 32-bit instead):

* "Visual Studio 10 2010 Win64"
* "Visual Studio 11 2012 Win64"
* "Visual Studio 12 2013 Win64"
* "Visual Studio 14 2015 Win64"
* "Visual Studio 15 2017 Win64"

For example you can build using a Visual Studio solution file on the command line by doing:

    C:\> CD workspace\build
    C:\workspace\build> "C:\Program Files\CMake\bin\cmake.exe" ..\thrift 
                          -DBISON_EXECUTABLE=..\thirdparty\dist\winflexbison\win_bison.exe
                          -DCMAKE_BUILD_TYPE=Release
                          -DFLEX_EXECUTABLE=..\thirdparty\dist\winflexbison\win_flex.exe
                          -DWITH_MT=ON
                          -DWITH_SHARED_LIB=OFF
                          -G"Visual Studio 14 2015 Win64"
    C:\workspace\build> MSBUILD "Apache Thrift.sln" /p:Configuration=Release /p:Platform=x64 /t:thrift-compiler

You can also double-click on the solution file to bring it up in Visual Studio and build or debug interactively from there.

## Building the thrift C++ Run-Time Library

These instructions are similar to the compiler build however there are additional dependencies on third party libraries to build a feature-complete runtime.  The resulting static link library for thrift uses a dynamic Microsoft runtime.

1. Open the desired Visual Studio Command Prompt.
2. `cd workspace`
3. `build-thrift.bat`

Thrift depends on boost, libevent, openssl, and zlib in order to build with all server and transport types.  To use later versions of boost like 1.62 you will need a recent version of cmake (at least 3.7).

The build-thrift script has options to build debug or release and to optionally disable any of the generation (cmake), build, or test phases.  By default, the batch file will generate an out-of-tree build directory inside `workspace\build`, then perform a release build, then run the unit tests.  The batch file accepts some option flags to control its behavior:

    :: Flags you can use to change this behavior:
    ::
    ::   /DEBUG            - if building, perform a debug build instead
    ::   /NOGENERATE       - skip cmake generation - useful if you
    ::                       have already generated a solution and just
    ::                       want to build
    ::   /NOBUILD          - skip cmake build - useful if you just
    ::                       want to generate a solution
    ::   /NOTEST           - skip ctest execution

For example if you want to generate the cmake environment without building or running tests:

    C:\workspace> build-thrift.bat /NOBUILD /NOTEST
