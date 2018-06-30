# Build and run compiler tests using CMake

<!-- TOC -->

- [Build and run compiler tests using CMake](#build-and-run-compiler-tests-using-cmake)
    - [General information](#general-information)
    - [How to add your tests](#how-to-add-your-tests)
    - [Build and run tests on Unix-like systems](#build-and-run-tests-on-unix-like-systems)
        - [Prerequisites:](#prerequisites)
        - [Build and run test with CMake](#build-and-run-test-with-cmake)
    - [Build and run tests on Windows](#build-and-run-tests-on-windows)
        - [Prerequisites:](#prerequisites-1)
        - [Generation of VS project with CMake, build and run on Windows](#generation-of-vs-project-with-cmake-build-and-run-on-windows)

<!-- /TOC -->

## General information 

Added generic way to cover code by tests for many languages (you just need to make a correct header file for generator for your language - example in **netcore** implementation)

At current moment these tests use free Catch library (https://github.com/catchorg/Catch2/tree/Catch1.x) for easy test creation and usage.
Decision to use it was because of simplicity, easy usage, one header file to use, stable community and growing interest  (https://cpp.libhunt.com/project/googletest-google/vs/catch?rel=cmp-cmp)

Also, maybe, later it will be migrated to Catch2 (https://github.com/philsquared/Catch) - depends on need to support legacy compilers (c++98)

## How to add your tests

- Open **CMakeLists.txt**
- Set **On** to call of **THRIFT_ADD_COMPILER** for your language

``` cmake 
THRIFT_ADD_COMPILER(netcore "Enable compiler for .NET Core" ON)
```

- Create folder with name specified in list of languages in **CMakeLists.txt**
- Create tests in folder for your language (with extensions like *.c* - cc, cpp, etc)
  - Don't forget to add include of catch.hpp in your test file
  ``` C
  #include "../catch/catch.hpp"
  ```

- If you need - add files manually to **thrift_compiler_tests_manual_SOURCES** in **CMakeLists.txt** similar to 

``` cmake
# you can add some files manually there 
set(thrift_compiler_tests_manual_SOURCES
    # tests file to avoid main in every test file
    ${CMAKE_CURRENT_SOURCE_DIR}/tests_main.cc
)
```

- Run **cmake** with arguments for your environment and compiler 
- Enjoy

## Build and run tests on Unix-like systems

### Prerequisites:
- Install CMake - <https://cmake.org/download/>
- Install winflexbison - <https://sourceforge.net/projects/winflexbison/>

### Build and run test with CMake

- Run commands in command line in current directory:

```
mkdir cmake-vs && cd cmake-vs
cmake ..
cmake --build .
ctest -C Debug -V
```

## Build and run tests on Windows

### Prerequisites:
- Install CMake - <https://cmake.org/download/>
- Install winflexbison - <https://sourceforge.net/projects/winflexbison/>
- Install VS2017 Community Edition - <https://www.visualstudio.com/vs/whatsnew/> (ensure that you installed workload "Desktop Development with C++" for VS2017)

### Generation of VS project with CMake, build and run on Windows
- Run commands in command line in current directory (ensure that VS installed):

```
mkdir cmake-vs
cd cmake-vs
cmake ..
cmake --build .
ctest -C Debug -V
```