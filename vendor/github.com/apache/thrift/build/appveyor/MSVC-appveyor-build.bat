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
MKDIR "%BUILDDIR%"                          || EXIT /B
CD "%BUILDDIR%"                             || EXIT /B

@ECHO ON
  cmake "%SRCDIR%" ^
    -G"%GENERATOR%" ^
	-DBISON_EXECUTABLE=C:\ProgramData\chocolatey\lib\winflexbison3\tools\win_bison.exe ^
    -DBOOST_ROOT="%BOOST_ROOT%" ^
    -DBOOST_LIBRARYDIR="%BOOST_LIBRARYDIR%" ^
    -DCMAKE_BUILD_TYPE="%CONFIGURATION%" ^
    -DCMAKE_INSTALL_PREFIX="%INSTDIR%" ^
	-DFLEX_EXECUTABLE=C:\ProgramData\chocolatey\lib\winflexbison3\tools\win_flex.exe ^
    -DINTTYPES_ROOT="%WIN3P%\msinttypes" ^
    -DLIBEVENT_ROOT="%WIN3P%\libevent-%LIBEVENT_VERSION%-stable" ^
    -DOPENSSL_ROOT_DIR="%OPENSSL_ROOT%" ^
    -DOPENSSL_USE_STATIC_LIBS=OFF ^
    -DZLIB_LIBRARY="%WIN3P%\zlib-inst\lib\zlib%ZLIB_LIB_SUFFIX%.lib" ^
    -DZLIB_ROOT="%WIN3P%\zlib-inst" ^
    -DWITH_PYTHON=%WITH_PYTHON% ^
    -DWITH_%THREADMODEL%THREADS=ON ^
    -DWITH_SHARED_LIB=OFF ^
    -DWITH_STATIC_LIB=ON                    || EXIT /B
@ECHO OFF

cmake --build . ^
  --config "%CONFIGURATION%" ^
  --target INSTALL                          || EXIT /B
