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

@ECHO ON
SETLOCAL EnableDelayedExpansion
CD build\appveyor              || EXIT /B
CALL cl_banner_test.bat        || EXIT /B
CALL cl_setenv.bat             || EXIT /B
CD "%BUILDDIR%"                || EXIT /B

DIR C:\libraries
DIR C:\libraries\boost_1_59_0
DIR C:\libraries\boost_1_60_0
DIR C:\libraries\boost_1_62_0
DIR C:\libraries\boost_1_63_0
DIR C:\libraries\boost_1_64_0

:: Add directories to the path to find DLLs of third party libraries so tests run properly!
SET PATH=%BOOST_LIBRARYDIR:/=\%;%OPENSSL_ROOT%\bin;%WIN3P%\zlib-inst\bin;%PATH%

ctest -C %CONFIGURATION% --timeout 300 -VV -E "(%DISABLED_TESTS%)" || EXIT /B
