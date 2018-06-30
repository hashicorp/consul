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

SET CMAKEARGS=^
  -G'%GENERATOR%' ^
  -DCMAKE_BUILD_TYPE=%CONFIGURATION% ^
  -DCMAKE_INSTALL_PREFIX=%INSTDIR% ^
  -DCMAKE_MAKE_PROGRAM=/mingw%NORM_PLATFORM%/bin/mingw32-make ^
  -DCMAKE_C_COMPILER=/mingw%NORM_PLATFORM%/bin/gcc.exe ^
  -DCMAKE_CXX_COMPILER=/mingw%NORM_PLATFORM%/bin/g++.exe ^
  -DWITH_PYTHON=OFF ^
  -DWITH_SHARED_LIB=OFF ^
  -DWITH_STATIC_LIB=ON

@ECHO ON
%BASH% -lc "mkdir -p %BUILDDIR% && cd %BUILDDIR% && cmake.exe %SRCDIR% %CMAKEARGS% && cmake --build . --config %CONFIGURATION% --target install" || EXIT /B
@ECHO OFF
