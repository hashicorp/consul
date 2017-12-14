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
:: Build script for zlib on windows.
:: Uses the environment set up by a Visual Studio Command Prompt shortcut
:: to target a specific architecture and compiler.
::

@ECHO OFF
SETLOCAL EnableDelayedExpansion

:: Sets variables for third party versions used in build
CALL ..\..\scripts\tpversions.bat || EXIT /B

:: use "build-zlib.bat /yes" to skip the question part
IF /I "%1" == "/YES"           SET NOASK=1

IF NOT DEFINED GENERATOR       SET GENERATOR=NMake Makefiles
IF NOT DEFINED PACKAGE_NAME    SET PACKAGE_NAME=zlib
IF NOT DEFINED PACKAGE_VERSION SET PACKAGE_VERSION=%TP_ZLIB_VERSION%
IF NOT DEFINED SOURCE_DIR      SET SOURCEDIR=%~dp0%PACKAGE_NAME%-%PACKAGE_VERSION%
IF NOT DEFINED WIN3P_ROOT      SET WIN3P_ROOT=%~dp0..

:: Set COMPILER to (vc100 - vc140) depending on the current environment
CALL ..\..\scripts\cl_setcompiler.bat || EXIT /B

:: Set ARCH to either win32 or x64 depending on the current environment
CALL ..\..\scripts\cl_setarch.bat || EXIT /B

FOR %%X IN (
  Debug 
  Release
) DO (
  SET BUILDTYPE=%%X
  SET BUILDDIR=%WIN3P_ROOT%\build\%PACKAGE_NAME%\%PACKAGE_VERSION%\%COMPILER%\%ARCH%\!BUILDTYPE!
  SET OUTDIR=%WIN3P_ROOT%\dist\%PACKAGE_NAME%-%PACKAGE_VERSION%\%COMPILER%\%ARCH%

  ECHO/
  ECHO =========================================================================
  ECHO          Building: %PACKAGE_NAME% v%PACKAGE_VERSION% %COMPILER%:%ARCH%:!BUILDTYPE! "%GENERATOR%"
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

  CMAKE.EXE -G"%GENERATOR%" -DCMAKE_INSTALL_PREFIX=!OUTDIR! -DCMAKE_BUILD_TYPE=!BUILDTYPE! "%SOURCEDIR%" || EXIT /B
  NMAKE /fMakefile install || EXIT /B

  IF "!BUILDTYPE!" == "Debug" (
    COPY "!BUILDDIR!\zlibd.pdb" "!OUTDIR!\bin\" || EXIT /B
  )
)

ENDLOCAL
