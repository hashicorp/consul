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

       IF "%PROFILE%" == "MSVC2010" (
  CALL "C:\Program Files (x86)\Microsoft Visual Studio 10.0\VC\vcvarsall.bat" %PLATFORM%
) ELSE IF "%PROFILE%" == "MSVC2012" (
  CALL "C:\Program Files (x86)\Microsoft Visual Studio 11.0\VC\vcvarsall.bat" %PLATFORM%
) ELSE IF "%PROFILE%" == "MSVC2013" (
  CALL "C:\Program Files (x86)\Microsoft Visual Studio 12.0\VC\vcvarsall.bat" %PLATFORM%
) ELSE IF "%PROFILE%" == "MSVC2015" (
  CALL "C:\Program Files (x86)\Microsoft Visual Studio 14.0\VC\vcvarsall.bat" %PLATFORM%
) ELSE IF "%PROFILE%" == "MSVC2017" (
  CALL :SETUPNEWERMSVC    || EXIT /B
) ELSE IF "%PROFILE%" == "MINGW" (
  REM Supported, nothing special to do here.
) ELSE IF "%PROFILE%" == "CYGWIN" (
  REM Supported, nothing special to do here.
) ELSE (
  ECHO Unsupported PROFILE=%PROFILE% or PLATFORM=%PLATFORM%
  EXIT /B 1
)

CALL cl_setcompiler.bat   || EXIT /B
CALL cl_setgenerator.bat  || EXIT /B

SET APPVEYOR_SCRIPTS=%APPVEYOR_BUILD_FOLDER%\build\appveyor
SET BUILDCACHE=%APPVEYOR_BUILD_FOLDER%\..\build\cache
SET BUILDDIR=%APPVEYOR_BUILD_FOLDER%\..\build\%PROFILE%\%PLATFORM%
SET INSTDIR=%APPVEYOR_BUILD_FOLDER%\..\build\%PROFILE%\%PLATFORM%
SET SRCDIR=%APPVEYOR_BUILD_FOLDER%

:: PLATFORM is x64 or x86
:: NORM_PLATFORM is 64 or 32
SET NORM_PLATFORM=%PLATFORM:~-2,2%
IF "%NORM_PLATFORM%" == "86" (SET NORM_PLATFORM=32)

IF "%PROFILE:~0,4%" == "MSVC" (

  :: FindBoost needs forward slashes so cmake doesn't see something as an escaped character
  SET BOOST_ROOT=C:/Libraries/boost_%BOOST_VERSION:.=_%
  SET BOOST_LIBRARYDIR=!BOOST_ROOT!/lib%NORM_PLATFORM%-msvc-%COMPILER:~-3,2%.%COMPILER:~-1,1%
  SET OPENSSL_ROOT=C:\OpenSSL-Win%NORM_PLATFORM%
  SET WIN3P=%APPVEYOR_BUILD_FOLDER%\thirdparty

  :: MSVC2010 doesn't "do" std::thread
  IF "%COMPILER%" == "vc100" (
    SET THREADMODEL=BOOST
  ) ELSE (
    SET THREADMODEL=STD
  )

  IF "%PYTHON_VERSION%" == "" (
    SET WITH_PYTHON=OFF
  ) ELSE (
    SET WITH_PYTHON=ON
    IF /i "%PLATFORM%" == "x64" SET PTEXT=-x64
    SET PATH=C:\Python%PYTHON_VERSION:.=%!PTEXT!\scripts;C:\Python%PYTHON_VERSION:.=%!PTEXT!;!PATH!
  )
  IF "%CONFIGURATION%" == "Debug" (SET ZLIB_LIB_SUFFIX=d)

  IF NOT "%QT_VERSION%" == "" (
    IF /i "%PLATFORM%" == "x64" SET QTEXT=_64
    SET PATH=C:\Qt\%QT_VERSION%\%PROFILE%!QTEXT!\bin;!PATH!
  )

) ELSE IF "%PROFILE:~0,4%" == "MING" (

  :: PLATFORM = x86 means MINGWPLAT i686
  :: PLATFORM = x64 means MINGWPLAT x86_64
  SET MINGWPLAT=x86_64
  IF "%PLATFORM%" == "x86" (SET MINGWPLAT=i686)

  SET BASH=C:\msys64\usr\bin\bash.exe
  !BASH! -lc "sed -i '/export PATH=\/mingw32\/bin/d' ~/.bash_profile && sed -i '/export PATH=\/mingw64\/bin/d' ~/.bash_profile && echo 'export PATH=/mingw%NORM_PLATFORM%/bin:$PATH' >> ~/.bash_profile" || EXIT /B

  SET BUILDDIR=%BUILDDIR:\=/%
  SET BUILDDIR=/c!BUILDDIR:~2!
  SET INSTDIR=%INSTDIR:\=/%
  SET INSTDIR=/c!INSTDIR:~2!
  SET SRCDIR=%SRCDIR:\=/%
  SET SRCDIR=/c!SRCDIR:~2!

) ELSE IF "%PROFILE:~0,4%" == "CYGW" (

  SET CYGWINROOT=C:\cygwin
  IF "%PLATFORM%" == "x64" (SET CYGWINROOT=!CYGWINROOT!64)

  SET BASH=!CYGWINROOT!\bin\bash.exe
  SET SETUP=!CYGWINROOT!\setup-x86
  IF "%PLATFORM%" == "x64" (SET SETUP=!SETUP!_64)
  SET SETUP=!SETUP!.exe

  SET BUILDDIR=%BUILDDIR:\=/%
  SET BUILDDIR=/cygdrive/c!BUILDDIR:~2!
  SET INSTDIR=%INSTDIR:\=/%
  SET INSTDIR_CYG=/cygdrive/c!INSTDIR:~2!
  SET SRCDIR=%SRCDIR:\=/%
  SET SRCDIR=/cygdrive/c!SRCDIR:~2!

)

GOTO :EOF

:SETUPNEWERMSVC
  FOR /F "USEBACKQ TOKENS=*" %%i IN (`call "%ProgramFiles(x86)%\Microsoft Visual Studio\Installer\vswhere.exe" -version "[15.0,16.0)" -property installationPath`) DO (
    IF "%MSVCROOT%" == "" (SET MSVCROOT=%%i)
  )
  SET MSVCPLAT=x86
  IF "%PLATFORM%" == "x64" (SET MSVCPLAT=amd64)

  SET CURRENTDIR=%CD%
  CALL "!MSVCROOT!\Common7\Tools\VsDevCmd.bat" -arch=!MSVCPLAT! || EXIT /B
  CD %CURRENTDIR%
  EXIT /B

:EOF
