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

       IF "%PROFILE%" == "MSVC2010" (
  CALL "C:\Program Files (x86)\Microsoft Visual Studio 10.0\VC\vcvarsall.bat" %PLATFORM%
) ELSE IF "%PROFILE%" == "MSVC2012" (
  CALL "C:\Program Files (x86)\Microsoft Visual Studio 11.0\VC\vcvarsall.bat" %PLATFORM%
) ELSE IF "%PROFILE%" == "MSVC2013" (
  CALL "C:\Program Files (x86)\Microsoft Visual Studio 12.0\VC\vcvarsall.bat" %PLATFORM%
) ELSE IF "%PROFILE%" == "MSVC2015" (
  CALL "C:\Program Files (x86)\Microsoft Visual Studio 14.0\VC\vcvarsall.bat" %PLATFORM%
) ELSE IF "%PROFILE%" == "MSVC2017" (
  CALL "C:\Program Files (x86)\Microsoft Visual Studio\2017\Enterprise\Common7\Tools\VsDevCmd.bat" %PLATFORM%
) ELSE IF "%PROFILE%" == "MINGW" (
  SET MSYS2_PATH_TYPE=stock
) ELSE IF "%PROFILE%" == "MSYS" (
  SET MSYS2_PATH_TYPE=stock
) ELSE (
  ECHO Unsupported PROFILE=%PROFILE% or PLATFORM=%PLATFORM%
  EXIT /B 1
)

CALL cl_setcompiler.bat   || EXIT /B
CALL cl_setgenerator.bat  || EXIT /B

SET APPVEYOR_SCRIPTS=%APPVEYOR_BUILD_FOLDER%\build\appveyor
SET BUILDCACHE=%APPVEYOR_BUILD_FOLDER%\buildcache
SET BUILDDIR=%APPVEYOR_BUILD_FOLDER%\local-thrift-build
SET INSTDIR=%APPVEYOR_BUILD_FOLDER%\local-thrift-inst
SET SRCDIR=%APPVEYOR_BUILD_FOLDER%

: PLATFORM is x64 or x86, but we want x86 to become "32" when we strip it down for paths:
SET NORM_PLATFORM=%PLATFORM:~-2,2%
IF "%NORM_PLATFORM%" == "86" (SET NORM_PLATFORM=32)

:: FindBoost needs forward slashes so cmake doesn't see something as an escaped character
SET BOOST_ROOT=C:/Libraries/boost_%BOOST_VERSION:.=_%
SET BOOST_LIBRARYDIR=%BOOST_ROOT%/lib%NORM_PLATFORM%-msvc-%COMPILER:~-3,2%.0
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
  SET PATH=C:\Python%PYTHON_VERSION:.=%\scripts;C:\Python%PYTHON_VERSION:.=%;!PATH!
)
IF "%CONFIGURATION%" == "Debug" (SET ZLIB_LIB_SUFFIX=d)

IF NOT "%QT_VERSION%" == "" (
  IF /i "%PLATFORM%" == "x64" SET QTEXT=_64
  SET PATH=C:\Qt\%QT_VERSION%\%PROFILE%!QTEXT!\bin;!PATH!
)

IF NOT "%PROFILE:~0,4%" == "MSVC" (

  SET BASH=C:\msys64\usr\bin\bash.exe
  SET BOOST_ROOT=
  SET BOOST_INCLUDEDIR=/mingw64/include
  SET BOOST_LIBRARYDIR=/mingw64/lib
  SET OPENSSL_LIBRARIES=/mingw64/lib
  SET OPENSSL_ROOT=/mingw64
  SET WIN3P=

  !BASH! -lc "sed -i '/export PATH=\/mingw64\/bin/d' ~/.bash_profile && echo 'export PATH=/mingw64/bin:$PATH' >> ~/.bash_profile" || EXIT /B

)

SET BUILDDIR_MSYS=%BUILDDIR:\=/%
SET BUILDDIR_MSYS=/c%BUILDDIR_MSYS:~2%
SET INSTDIR_MSYS=%INSTDIR:\=/%
SET INSTDIR_MSYS=/c%INSTDIR_MSYS:~2%
SET SRCDIR_MSYS=%SRCDIR:\=/%
SET SRCDIR_MSYS=/c%SRCDIR_MSYS:~2%
