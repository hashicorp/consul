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
:: Produces a production thrift compiler suitable for redistribution.
:: The compiler is linked to runtime statically for maximum portability.
:: Assumes the thirdparty files for "winflexbison" have been placed
:: according to the README.md instructions.
::
:: Open a Visual Studio Command Prompt of your choosing and then
:: run this script.  

@ECHO OFF
SETLOCAL EnableDelayedExpansion

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

IF NOT DEFINED BUILDTYPE (
  SET BUILDTYPE=Release
)

  SET BUILDDIR=%~dp0build\%PACKAGE_NAME%-compiler\%PACKAGE_VERSION%\%COMPILER%\
  SET OUTDIR=%~dp0dist\%PACKAGE_NAME%-compiler-%PACKAGE_VERSION%\%COMPILER%\%ARCH%\%BUILDTYPE%\
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

  MKDIR "%BUILDDIR%"
  CD "%BUILDDIR%" || EXIT /B

  CMAKE.EXE %~dp0thrift ^
    -G"%GENERATOR%" ^
    -DBISON_EXECUTABLE=%WIN3P_ROOT%\dist\winflexbison\win_bison.exe ^
    -DCMAKE_BUILD_TYPE=%BUILDTYPE% ^
    -DFLEX_EXECUTABLE=%WIN3P_ROOT%\dist\winflexbison\win_flex.exe ^
    -DWITH_MT=ON ^
    -DWITH_SHARED_LIB=OFF || EXIT /B

  CD %BUILDDIR%

  CMAKE.EXE --build . --config %BUILDTYPE% --target thrift-compiler || EXIT /B
  XCOPY /F /Y %BUILDDIR%\bin\%BUILDTYPE%\thrift.exe %OUTDIR%

ENDLOCAL
EXIT /B
