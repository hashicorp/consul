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
:: Build script for libevent on windows
:: Use libevent master from github which has cmake integration
:: Uses the environment set up by a Visual Studio Command Prompt shortcut
:: to target a specific architecture and compiler
::
:: Creates a static link library.
:: Links against OpenSSL and zlib statically.
::

@ECHO OFF
SETLOCAL EnableDelayedExpansion

:: Sets variables for third party versions used in build
CALL ..\..\scripts\tpversions.bat || EXIT /B

:: use "build-libevent.bat /yes" to skip the question part
IF /I "%1" == "/YES"           SET NOASK=1

:: Set COMPILER to (vc100 - vc140) depending on the current environment
CALL ..\..\scripts\cl_setcompiler.bat || EXIT /B

:: Set ARCH to either win32 or x64 depending on the current environment
CALL ..\..\scripts\cl_setarch.bat || EXIT /B

IF NOT DEFINED GENERATOR       SET GENERATOR=NMake Makefiles
IF NOT DEFINED PACKAGE_NAME    SET PACKAGE_NAME=libevent
IF NOT DEFINED PACKAGE_VERSION SET PACKAGE_VERSION=%TP_LIBEVENT_VERSION%
IF NOT DEFINED SOURCEDIR       SET SOURCEDIR=%~dp0%PACKAGE_NAME%-%PACKAGE_VERSION%
IF NOT DEFINED WIN3P_ROOT      SET WIN3P_ROOT=%~dp0..

FOR %%X IN (
  Debug 
  Release
) DO (
  SET BUILDTYPE=%%X
  SET BUILDDIR=%WIN3P_ROOT%\build\%PACKAGE_NAME%\%PACKAGE_VERSION%\%COMPILER%\%ARCH%\!BUILDTYPE!
  SET OUTDIR=%WIN3P_ROOT%\dist\%PACKAGE_NAME%-%PACKAGE_VERSION%\%COMPILER%\%ARCH%\!BUILDTYPE!

  IF "!BUILDTYPE!" == "Debug" (SET ZLIB_LIB_SUFFIX=d)

  SET CMAKE_DEFS=^
   -DEVENT__DISABLE_SAMPLES=ON ^
   -DEVENT__DISABLE_TESTS=ON ^
   -DOPENSSL_USE_STATIC_LIBS=OFF ^
   -DOPENSSL_ROOT_DIR=%WIN3P_ROOT%\dist\openssl-%TP_OPENSSL_VERSION%\%COMPILER%\%ARCH%\!BUILDTYPE!\dynamic ^
   -DZLIB_LIBRARY=%WIN3P_ROOT%\dist\zlib-%TP_ZLIB_VERSION%\%COMPILER%\%ARCH%\lib\zlib!ZLIB_LIB_SUFFIX!.lib ^
   -DZLIB_ROOT=%WIN3P_ROOT%\dist\zlib-%TP_ZLIB_VERSION%\%COMPILER%\%ARCH%

  ECHO/
  ECHO =========================================================================
  ECHO          Building: %PACKAGE_NAME% v%PACKAGE_VERSION% %COMPILER%:%ARCH%:!BUILDTYPE! "%GENERATOR%"
  ECHO CMake Definitions: !CMAKE_DEFS!
  ECHO   Build Directory: !BUILDDIR!
  ECHO Install Directory: !OUTDIR!
  ECHO  Source Directory: %SOURCEDIR%
  ECHO =========================================================================
  ECHO/

  IF NOT DEFINED NOASK (
    CHOICE /M "Do you want to build this configuration? " /c YN
    IF !ERRORLEVEL! NEQ 1 (EXIT /B !ERRORLEVEL!)
  )
  
  MKDIR "!BUILDDIR!"
  CD "!BUILDDIR!" || EXIT /B

  CMAKE.EXE -G"%GENERATOR%" -DCMAKE_INSTALL_PREFIX=!OUTDIR! -DCMAKE_BUILD_TYPE=!BUILDTYPE! !CMAKE_DEFS! "%SOURCEDIR%" || EXIT /B
  NMAKE /fMakefile install || EXIT /B
)

ENDLOCAL
