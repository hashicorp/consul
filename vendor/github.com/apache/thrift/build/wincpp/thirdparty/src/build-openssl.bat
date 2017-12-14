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
:: Build script for openssl on windows
:: openssl uses an in-tree build so you have to clean between each one
::
:: Uses the environment set up by a Visual Studio Command Prompt shortcut
:: to target a specific architecture and compiler
::
:: If you use Lavasoft Ad-Aware, disable it for this build.  It blocks the creation
:: of any file named "clienthellotest.exe" for whatever reason, which breaks the build.
::

@ECHO OFF
SETLOCAL EnableDelayedExpansion

:: Sets variables for third party versions used in build
CALL ..\..\scripts\tpversions.bat || EXIT /B

:: use "build-openssl.bat /yes" to skip the question part
IF /I "%1" == "/YES"           SET NOASK=1

IF NOT DEFINED PACKAGE_NAME    SET PACKAGE_NAME=openssl
IF NOT DEFINED PACKAGE_VERSION SET PACKAGE_VERSION=%TP_OPENSSL_VERSION%
IF NOT DEFINED SOURCEDIR       SET SOURCEDIR=%~dp0%PACKAGE_NAME%-%PACKAGE_VERSION%
IF NOT DEFINED WIN3P_ROOT      SET WIN3P_ROOT=%~dp0..

:: Set COMPILER to (vc100 - vc140) depending on the current environment
CALL ..\..\scripts\cl_setcompiler.bat || EXIT /B

:: Set ARCH to either win32 or x64 depending on the current environment
CALL ..\..\scripts\cl_setarch.bat || EXIT /B

IF "%ARCH%" == "x64" (
  SET TODO=debug-VC-WIN64A VC-WIN64A
) ELSE (
  SET TODO=debug-VC-WIN32 VC-WIN32
)

FOR %%X IN ( !TODO! ) DO (
  SET BUILDTYPE=%%X
  FOR %%Y IN (
    nt
    ntdll
  ) DO (
    SET LIBTYPE=%%Y

    IF "!BUILDTYPE:~0,6!" == "debug-" (
      SET OUTBUILDTYPE=debug
      SET ZLIBLIBSUFFIX=d
    ) ELSE (
      SET OUTBUILDTYPE=release
      SET ZLIBLIBSUFFIX=
    )

    IF "!LIBTYPE!" == "ntdll" (
      SET BUILD_OPTIONS=shared
      SET OUTLIBTYPE=dynamic
      SET ZLIBLIB=zlib!ZLIBLIBSUFFIX!
      SET ZLIBOPT=zlib-dynamic
    ) ELSE (
      SET BUILD_OPTIONS=no-shared
      SET OUTLIBTYPE=static
      SET ZLIBLIB=zlibstatic!ZLIBLIBSUFFIX!.lib
      SET ZLIBOPT=zlib
    )

    SET LIB=%~dp0..\dist\zlib-%TP_ZLIB_VERSION%\!COMPILER!\!ARCH!\lib;!LIB!
    SET BUILD_OPTIONS=!BUILD_OPTIONS! no-asm no-unit-test !ZLIBOPT! --openssldir=ssl --with-zlib-include=%~dp0..\dist\zlib-%TP_ZLIB_VERSION%\!COMPILER!\!ARCH!\include --with-zlib-lib=!ZLIBLIB!
    SET OUTDIR=%WIN3P_ROOT%\dist\%PACKAGE_NAME%-%PACKAGE_VERSION%\%COMPILER%\%ARCH%\!OUTBUILDTYPE!\!OUTLIBTYPE!

    ECHO/
    ECHO =========================================================================
    ECHO          Building: %PACKAGE_NAME% %PACKAGE_VERSION% %COMPILER%:%ARCH%:!OUTBUILDTYPE!:!OUTLIBTYPE! [!BUILDTYPE!]
    ECHO Configure Options: !BUILD_OPTIONS!
    ECHO Install Directory: !OUTDIR!
    ECHO  Source Directory: %SOURCEDIR%
    ECHO =========================================================================
    ECHO/

    IF NOT DEFINED NOASK (
      CHOICE /M "Do you want to build this configuration? " /c YN
      IF !ERRORLEVEL! NEQ 1 (EXIT /B !ERRORLEVEL!)
    )

    CD %SOURCEDIR% || EXIT /B
    perl Configure !BUILDTYPE! --prefix="!OUTDIR!" !BUILD_OPTIONS! || EXIT /B
    NMAKE /FMakefile install_sw || EXIT /B
    NMAKE /FMakefile clean || EXIT /B
  )
)

ENDLOCAL
EXIT /B
