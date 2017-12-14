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
:: Appveyor install script for MSVC
:: Installs (or builds) third party packages we need
::

@ECHO OFF
SETLOCAL EnableDelayedExpansion

CD build\appveyor                         || EXIT /B
CALL cl_banner_install.bat                || EXIT /B
CALL cl_setenv.bat                        || EXIT /B
CALL cl_showenv.bat                       || EXIT /B
MKDIR "%WIN3P%"                           || EXIT /B

:: Install ant - this also installs the latest JDK as a dependency
:: The installation of JDK requires us to pick up PATH and JAVE_HOME from the registry
cinst -c "%BUILDCACHE%" -y ant            || EXIT /B

:: Install bison and flex
cinst -c "%BUILDCACHE%" -y winflexbison3  || EXIT /B

:: zlib
CD "%APPVEYOR_SCRIPTS%"                   || EXIT /B
call build-zlib.bat                       || EXIT /B

:: libevent
CD "%APPVEYOR_SCRIPTS%"                   || EXIT /B
call build-libevent.bat                   || EXIT /B

:: python packages
pip install backports.ssl_match_hostname ^
            ipaddress ^
            tornado ^
            twisted                       || EXIT /B

:: msinttypes - for MSVC2010 only
SET MSINTTYPESURL=https://storage.googleapis.com/google-code-archive-downloads/v2/code.google.com/msinttypes/msinttypes-r26.zip
IF "%COMPILER%" == "vc100" (
  MKDIR "%WIN3P%\msinttypes"              || EXIT /B
  CD "%WIN3P%\msinttypes"                 || EXIT /B
  appveyor DownloadFile "%MSINTTYPESURL%" || EXIT /B
  7z x "msinttypes-r26.zip"               || EXIT /B
)

:: appveyor build slaves do not have MSVC2010 Boost installed
IF "%COMPILER%" == "vc100" (
  SET BITS=64
  IF "%PLATFORM%" == "x86" (
    SET BITS=32
  )
  SET BOOSTEXEURL=https://downloads.sourceforge.net/project/boost/boost-binaries/%BOOST_VERSION%/boost_%BOOST_VERSION:.=_%-msvc-10.0-!BITS!.exe
  SET BOOSTEXE=C:\projects\thrift\buildcache\boost_%BOOST_VERSION:.=_%-msvc-10.0-!BITS!.exe
  appveyor DownloadFile "!BOOSTEXEURL!" -FileName "!BOOSTEXE!" || EXIT /B
  "!BOOSTEXE!" /dir=C:\Libraries\boost_%BOOST_VERSION:.=_% /silent || EXIT /B
)

:: Haskell (GHC) and cabal
cinst -c "%BUILDCACHE%" -y ghc            || EXIT /B
