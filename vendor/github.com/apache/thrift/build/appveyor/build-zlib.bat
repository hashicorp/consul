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

SETLOCAL EnableDelayedExpansion

SET PACKAGE=zlib-%ZLIB_VERSION%
SET BUILDDIR=%WIN3P%\zlib-build
SET INSTDIR=%WIN3P%\zlib-inst
SET SRCDIR=%WIN3P%\%PACKAGE%
SET URLFILE=%PACKAGE%.tar.gz

:: This allows us to tolerate when the current version is archived
SET URL=http://zlib.net/%URLFILE%
SET FURL=http://zlib.net/fossils/%URLFILE%

:: Download
CD "%WIN3P%"                                                     || EXIT /B
appveyor DownloadFile "%URL%"
IF ERRORLEVEL 1 (
    appveyor DownloadFile "%FURL%"                               || EXIT /B
)
7z x "%URLFILE%" -so | 7z x -si -ttar > nul                      || EXIT /B

:: Generate
MKDIR "%BUILDDIR%"                                               || EXIT /B
CD "%BUILDDIR%"                                                  || EXIT /B
cmake "%SRCDIR%" ^
      -G"NMake Makefiles" ^
      -DCMAKE_INSTALL_PREFIX="%INSTDIR%" ^
      -DCMAKE_BUILD_TYPE="%CONFIGURATION%"                       || EXIT /B

:: Build
nmake /fMakefile install                                         || EXIT /B
IF "%CONFIGURATION%" == "Debug" (
    COPY "%BUILDDIR%\zlibd.pdb" "%INSTDIR%\bin\"                 || EXIT /B
)

ENDLOCAL
