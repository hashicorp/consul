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

::
:: Appveyor install script for MSYS
:: Installs (or builds) third party packages we need
::

@ECHO OFF
SETLOCAL EnableDelayedExpansion

CD build\appveyor                          || EXIT /B
CALL cl_banner_install.bat                 || EXIT /B
CALL cl_setenv.bat                         || EXIT /B
CALL cl_showenv.bat                        || EXIT /B

SET PACKAGES=^
  --needed -S bison flex ^
  make ^
  mingw-w64-x86_64-boost ^
  mingw-w64-x86_64-cmake ^
  mingw-w64-x86_64-openssl ^
  mingw-w64-x86_64-toolchain ^
  mingw-w64-x86_64-zlib

:: omitting libevent-devel for now it is version 2.1.4 and doesn't play nice with MinGW

%BASH% -lc "pacman --noconfirm -Syu"       || EXIT /B
%BASH% -lc "pacman --noconfirm -Su"        || EXIT /B
%BASH% -lc "pacman --noconfirm %PACKAGES%" || EXIT /B
