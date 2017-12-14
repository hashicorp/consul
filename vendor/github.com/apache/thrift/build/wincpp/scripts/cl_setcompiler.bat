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
:: Detect the compiler edition we're building in.
:: Set the COMPILER environment variable to one of:
::   vc100 = Visual Studio 2010
::   vc110 = Visual Studio 2012
::   vc120 = Visual Studio 2013
::   vc140 = Visual Studio 2015
::   vc150 = Visual Studio 2017
::
:: Honors any existing COMPILER environment variable
::   setting instead of overwriting it, to allow it
::   to be forced if needed.
::
:: Sets ERRORLEVEL to 0 if COMPILER can be determined,
::                 to 1 if it cannot.
::

IF DEFINED COMPILER (
  ECHO [warn ] using existing environment variable COMPILER
  EXIT /B 0
)

CALL :CHECK 16
IF %ERRORLEVEL% == 0 (IF NOT DEFINED COMPILER SET COMPILER=vc100)
CALL :CHECK 17
IF %ERRORLEVEL% == 0 (IF NOT DEFINED COMPILER SET COMPILER=vc110)
CALL :CHECK 18
IF %ERRORLEVEL% == 0 (IF NOT DEFINED COMPILER SET COMPILER=vc120)
CALL :CHECK 19.00
IF %ERRORLEVEL% == 0 (IF NOT DEFINED COMPILER SET COMPILER=vc140)
CALL :CHECK 19.10
IF %ERRORLEVEL% == 0 (IF NOT DEFINED COMPILER SET COMPILER=vc150)

IF NOT DEFINED COMPILER (
  ECHO [error] unable to determine the compiler edition
  EXIT /B 1
)

ECHO [info ] detected compiler edition    %COMPILER%
EXIT /B 0

:CHECK
cl /? 2>&1 | findstr /C:"Version %1%." > nul
EXIT /B %ERRORLEVEL%
