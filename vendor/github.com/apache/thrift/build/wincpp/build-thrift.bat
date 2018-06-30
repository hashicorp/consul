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
:: Generates a Visual Studio solution for thrift and then builds it.
:: Assumes third party libraries have been built or placed already.
::
:: Open a Visual Studio Command Prompt of your choosing and then
:: run this script.  
::
:: Normally the script will run cmake to generate a solution, then
:: perform a build, then run tests on the complete thrift library
:: in release mode.
::
:: Flags you can use to change this behavior:
::
::   /DEBUG            - debug instead of release
::   /IDE              - launch Visual Studio with a path set
::                       up correctly to run tests instead of
::                       performing any other actions, i.e.
::                       implies setting the next three flags
::   /NOGENERATE       - skip cmake generation - useful if you
::                       have already generated a solution and just
::                       want to build
::   /NOBUILD          - skip cmake build - useful if you just
::                       want to generate a solution
::   /NOTEST           - skip ctest execution
::

@ECHO OFF
SETLOCAL EnableDelayedExpansion

:: Sets variables for third party versions used in build
CALL scripts\tpversions.bat || EXIT /B

IF NOT DEFINED PACKAGE_NAME    SET PACKAGE_NAME=thrift
IF NOT DEFINED PACKAGE_VERSION SET PACKAGE_VERSION=dev
IF NOT DEFINED SOURCE_DIR      SET SOURCEDIR=%~dp0%PACKAGE_NAME%
IF NOT DEFINED WIN3P_ROOT      SET WIN3P_ROOT=%~dp0thirdparty

:: Set COMPILER to (vc100 - vc140) depending on the current environment
CALL scripts\cl_setcompiler.bat || EXIT /B

:: Set ARCH to either win32 or x64 depending on the current environment
CALL scripts\cl_setarch.bat || EXIT /B

:: Set GENERATOR for CMake depending on the current environment
CALL scripts\cl_setgenerator.bat || EXIT /B

:: Defaults

IF NOT DEFINED BUILDTYPE SET BUILDTYPE=Release
SET OPT_IDE=0
SET OPT_BUILD=1
SET OPT_GENERATE=1
SET OPT_TEST=1

:: Apply Flags

IF /I "%1" == "/DEBUG"        SET BUILDTYPE=Debug
IF /I "%2" == "/DEBUG"        SET BUILDTYPE=Debug
IF /I "%3" == "/DEBUG"        SET BUILDTYPE=Debug
IF /I "%1" == "/IDE"          SET OPT_IDE=1
IF /I "%2" == "/IDE"          SET OPT_IDE=1
IF /I "%3" == "/IDE"          SET OPT_IDE=1
IF /I "%1" == "/NOBUILD"      SET OPT_BUILD=0
IF /I "%2" == "/NOBUILD"      SET OPT_BUILD=0
IF /I "%3" == "/NOBUILD"      SET OPT_BUILD=0
IF /I "%1" == "/NOGENERATE"   SET OPT_GENERATE=0
IF /I "%2" == "/NOGENERATE"   SET OPT_GENERATE=0
IF /I "%3" == "/NOGENERATE"   SET OPT_GENERATE=0
IF /I "%1" == "/NOTEST"       SET OPT_TEST=0
IF /I "%2" == "/NOTEST"       SET OPT_TEST=0
IF /I "%3" == "/NOTEST"       SET OPT_TEST=0

IF %OPT_IDE% == 1 (
  SET OPT_GENERATE=0
  SET OPT_BUILD=0
  SET OPT_TEST=0
)

  SET BUILDDIR=%~dp0build\%PACKAGE_NAME%\%PACKAGE_VERSION%\%COMPILER%\%ARCH%\
  SET OUTDIR=%~dp0dist\%PACKAGE_NAME%-%PACKAGE_VERSION%\%COMPILER%\%ARCH%\%BUILDTYPE%\
  SET BOOST_LIBDIR=lib%ARCH:~-2,2%-msvc-%COMPILER:~-3,2%.0
  IF "%BUILDTYPE%" == "Debug" (SET ZLIB_STATIC_SUFFIX=d)

  ECHO/
  ECHO =========================================================================
  ECHO     Configuration: %PACKAGE_NAME% %PACKAGE_VERSION% %COMPILER%:%ARCH%:%BUILDTYPE% "%GENERATOR%"
IF DEFINED COMPILERONLY (
  ECHO                    COMPILER ONLY
)
  ECHO   Build Directory: %BUILDDIR%
  ECHO Install Directory: %OUTDIR%
  ECHO  Source Directory: %SOURCEDIR%
  ECHO =========================================================================
  ECHO/

IF %OPT_IDE% == 1 (

  CALL :SETRUNPATH || EXIT /B
  CALL DEVENV "!BUILDDIR!Apache Thrift.sln" || EXIT /B
  EXIT /B
  
)

  MKDIR "%BUILDDIR%"
  CD "%BUILDDIR%" || EXIT /B

IF %OPT_GENERATE% == 1 (

  CMAKE.EXE %~dp0thrift ^
    -G"%GENERATOR%" ^
    -DBISON_EXECUTABLE=%WIN3P_ROOT%\dist\winflexbison\win_bison.exe ^
    -DBOOST_ROOT=%WIN3P_ROOT%\dist\boost_%TP_BOOST_VERSION% ^
    -DBOOST_LIBRARYDIR=%WIN3P_ROOT%\dist\boost_%TP_BOOST_VERSION%\%BOOST_LIBDIR% ^
    -DCMAKE_INSTALL_PREFIX=%OUTDIR% ^
    -DCMAKE_BUILD_TYPE=%BUILDTYPE% ^
    -DFLEX_EXECUTABLE=%WIN3P_ROOT%\dist\winflexbison\win_flex.exe ^
    -DINTTYPES_ROOT=%WIN3P_ROOT%\dist\msinttypes ^
    -DLIBEVENT_ROOT=%WIN3P_ROOT%\dist\libevent-%TP_LIBEVENT_VERSION%\%COMPILER%\%ARCH%\%BUILDTYPE% ^
    -DOPENSSL_ROOT_DIR=%WIN3P_ROOT%\dist\openssl-%TP_OPENSSL_VERSION%\%COMPILER%\%ARCH%\%BUILDTYPE%\dynamic ^
    -DOPENSSL_USE_STATIC_LIBS=OFF ^
    -DZLIB_LIBRARY=%WIN3P_ROOT%\dist\zlib-%TP_ZLIB_VERSION%\%COMPILER%\%ARCH%\lib\zlib%ZLIB_LIB_SUFFIX%.lib ^
    -DZLIB_ROOT=%WIN3P_ROOT%\dist\zlib-%TP_ZLIB_VERSION%\%COMPILER%\%ARCH% ^
    -DWITH_BOOSTTHREADS=ON ^
    -DWITH_SHARED_LIB=OFF ^
    -DWITH_STATIC_LIB=ON || EXIT /B

)

IF %OPT_BUILD% == 1 (

  CD %BUILDDIR%
  CMAKE.EXE --build . --config %BUILDTYPE% --target INSTALL || EXIT /B

)

IF %OPT_TEST% == 1 (

  CALL :SETRUNPATH || EXIT /B
  CMAKE.EXE --build . --config %BUILDTYPE% --target RUN_TESTS || EXIT /B
  
)

:SETRUNPATH
  SET PATH=!PATH!;%WIN3P_ROOT%\dist\boost_%TP_BOOST_VERSION%\%BOOST_LIBDIR%
  SET PATH=!PATH!;%WIN3P_ROOT%\dist\openssl-%TP_OPENSSL_VERSION%\%COMPILER%\%ARCH%\%BUILDTYPE%\dynamic\bin
  SET PATH=!PATH!;%WIN3P_ROOT%\dist\zlib-%TP_ZLIB_VERSION%\%COMPILER%\%ARCH%\bin
  EXIT /B

ENDLOCAL
EXIT /B
