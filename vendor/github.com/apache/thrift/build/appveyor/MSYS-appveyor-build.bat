::
:: Licensed under the Apache License, Version 2.0 (the "License");
:: you may not use this file except in compliance with the License.
:: You may obtain a copy of the License at
::
::     http://www.apache.org/licenses/LICENSE-2.0
::
:: Unless required by applicable law or agreed to in writing, software
:: distributed under the License is distributed on an "AS IS" BASIS,
:: WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
:: See the License for the specific language governing permissions and
:: limitations under the License.
::

@ECHO OFF
SETLOCAL EnableDelayedExpansion

CD build\appveyor                           || EXIT /B
CALL cl_banner_build.bat                    || EXIT /B
CALL cl_setenv.bat                          || EXIT /B

SET BASH=C:\msys64\usr\bin\bash
SET CMAKE=/c/msys64/mingw64/bin/cmake.exe

@ECHO ON
SET CMAKEARGS=-G\"%GENERATOR%\" ^
  -DBoost_DEBUG=ON ^
  -DBoost_NAMESPACE=libboost ^
  -DBOOST_INCLUDEDIR=%BOOST_INCLUDEDIR% ^
  -DBOOST_LIBRARYDIR=%BOOST_LIBRARYDIR% ^
  -DCMAKE_BUILD_TYPE=%CONFIGURATION% ^
  -DCMAKE_C_COMPILER=gcc.exe ^
  -DCMAKE_CXX_COMPILER=g++.exe ^
  -DCMAKE_MAKE_PROGRAM=make.exe ^
  -DCMAKE_INSTALL_PREFIX=%INSTDIR_MSYS% ^
  -DOPENSSL_LIBRARIES=%OPENSSL_LIBRARIES% ^
  -DOPENSSL_ROOT_DIR=%OPENSSL_ROOT% ^
  -DOPENSSL_USE_STATIC_LIBS=ON ^
  -DWITH_BOOST_STATIC=ON ^
  -DWITH_JAVA=OFF ^
  -DWITH_LIBEVENT=OFF ^
  -DWITH_PYTHON=%WITH_PYTHON% ^
  -DWITH_SHARED_LIB=OFF ^
  -DWITH_STATIC_LIB=ON

%BASH% -lc "mkdir %BUILDDIR_MSYS% && cd %BUILDDIR_MSYS% && %CMAKE% %SRCDIR_MSYS% %CMAKEARGS% && %CMAKE% --build . --config %CONFIGURATION% --target install" || EXIT /B
@ECHO OFF
