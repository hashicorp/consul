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

ECHO/
ECHO ===============================================================================
IF "%PROFILE:~0,4%" == "MSVC" (
ECHO Versions
ECHO -------------------------------------------------------------------------------
ECHO boost                 = %BOOST_VERSION%
ECHO libevent              = %LIBEVENT_VERSION%
ECHO python                = %PYTHON_VERSION%
ECHO qt                    = %QT_VERSION%
ECHO zlib                  = %ZLIB_VERSION%
ECHO/
)
ECHO Appveyor Variables
ECHO -------------------------------------------------------------------------------
ECHO APPVEYOR_BUILD_FOLDER = %APPVEYOR_BUILD_FOLDER%
ECHO CONFIGURATION         = %CONFIGURATION%
ECHO PLATFORM              = %PLATFORM%
ECHO PROFILE               = %PROFILE%
ECHO/
ECHO Our Variables
ECHO -------------------------------------------------------------------------------
ECHO APPVEYOR_SCRIPTS      = %APPVEYOR_SCRIPTS%
ECHO BASH                  = %BASH%
ECHO BOOST_ROOT            = %BOOST_ROOT%
ECHO BOOST_INCLUDEDIR      = %BOOST_INCLUDEDIR%
ECHO BOOST_LIBRARYDIR      = %BOOST_LIBRARYDIR%
ECHO BUILDCACHE            = %BUILDCACHE%
ECHO BUILDDIR              = %BUILDDIR%
ECHO COMPILER              = %COMPILER%
ECHO GENERATOR             = %GENERATOR%
ECHO INSTDIR               = %INSTDIR%
ECHO JAVA_HOME             = %JAVA_HOME%
ECHO OPENSSL_ROOT          = %OPENSSL_ROOT%
ECHO SETUP                 = %SETUP%
ECHO SRCDIR                = %SRCDIR%
ECHO WIN3P                 = %WIN3P%
ECHO WITH_PYTHON           = %WITH_PYTHON%
ECHO ZLIB_STATIC_SUFFIX    = %ZLIB_STATIC_SUFFIX%
IF NOT "%PROFILE:~0,4%" == "MSVC" (
  ECHO/
  ECHO UNIXy PATH
  ECHO -------------------------------------------------------------------------------
  %BASH% -lc "echo $PATH"
)
ECHO/
ECHO Windows PATH
ECHO -------------------------------------------------------------------------------
ECHO %PATH%
ECHO ===============================================================================
ECHO/
