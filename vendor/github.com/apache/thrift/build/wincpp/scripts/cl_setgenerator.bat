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
:: Detect the compiler edition we're building in and then 
:: set the GENERATOR environment variable to one of:
::
::  Visual Studio 15 2017 [arch] = Generates Visual Studio 2017 project files.
::                                 Optional [arch] can be "Win64" or "ARM".
::  Visual Studio 14 2015 [arch] = Generates Visual Studio 2015 project files.
::                                 Optional [arch] can be "Win64" or "ARM".
::  Visual Studio 12 2013 [arch] = Generates Visual Studio 2013 project files.
::                                 Optional [arch] can be "Win64" or "ARM".
::  Visual Studio 11 2012 [arch] = Generates Visual Studio 2012 project files.
::                                 Optional [arch] can be "Win64" or "ARM".
::  Visual Studio 10 2010 [arch] = Generates Visual Studio 2010 project files.
::                                 Optional [arch] can be "Win64" or "IA64".
::
:: Honors any existing GENERATOR environment variable
::   setting instead of overwriting it, to allow it
::   to be forced if needed.
::
:: Sets ERRORLEVEL to 0 if GENERATOR can be determined,
::                 to 1 if it cannot.
::
:: Requires cl_setarch.bat to have been executed or the ARCH environment
:: variable to be set.
::

IF "%ARCH%" == "x64" (SET GENARCH= Win64)

IF DEFINED GENERATOR (
  ECHO [warn ] using existing environment variable GENERATOR
  EXIT /B 0
)

CALL :CHECK 16
IF %ERRORLEVEL% == 0 (IF NOT DEFINED GENERATOR SET GENERATOR=Visual Studio 10 2010%GENARCH%)
CALL :CHECK 17
IF %ERRORLEVEL% == 0 (IF NOT DEFINED GENERATOR SET GENERATOR=Visual Studio 11 2012%GENARCH%)
CALL :CHECK 18
IF %ERRORLEVEL% == 0 (IF NOT DEFINED GENERATOR SET GENERATOR=Visual Studio 12 2013%GENARCH%)
CALL :CHECK 19.00
IF %ERRORLEVEL% == 0 (IF NOT DEFINED GENERATOR SET GENERATOR=Visual Studio 14 2015%GENARCH%)
CALL :CHECK 19.10
IF %ERRORLEVEL% == 0 (IF NOT DEFINED GENERATOR SET GENERATOR=Visual Studio 15 2017%GENARCH%)

IF NOT DEFINED GENERATOR (
  ECHO [error] unable to determine the CMake generator to use
  EXIT /B 1
)

ECHO [info ] using CMake generator        %GENERATOR%
EXIT /B 0

:CHECK
cl /? 2>&1 | findstr /C:"Version %1%." > nul
EXIT /B %ERRORLEVEL%
