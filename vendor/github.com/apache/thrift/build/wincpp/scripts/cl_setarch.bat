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
:: Detect the architecture we're building for.
:: Set the ARCH environment variable to one of:
::   win32
::   x64
::
:: Honors any existing ARCH environment variable
::   setting instead of overwriting it, to allow it
::   to be forced if needed.
::
:: Sets ERRORLEVEL to 0 if ARCH can be determined,
::                 to 1 if it cannot.
::

IF DEFINED ARCH (
  ECHO [warn ] using existing environment variable ARCH
  EXIT /B 0
)

CALL :CHECK x64
IF %ERRORLEVEL% == 0 (SET ARCH=x64) ELSE (SET ARCH=win32)

IF NOT DEFINED ARCH (
  ECHO [error] unable to determine the target architecture
  EXIT /B 1
)

ECHO [info ] detected target architecture %ARCH%
EXIT /B 0

:CHECK
cl /? 2>&1 | findstr /C:" for %1%" > nul
EXIT /B %ERRORLEVEL%
