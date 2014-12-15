@echo off

REM Download Mingw 64 on Windows from http://win-builds.org/download.html

set GOARCH=%2
IF "%2" == "" (set GOARCH=amd64)
set MODULENAME=%1
set ORG_PATH=github.com\hashicorp
set REPO_PATH=%ORG_PATH%\%MODULENAME%

set GOPATH=%cd%\gopath

rmdir /s /q %GOPATH%\src\%REPO_PATH% 2>nul
mkdir %GOPATH%\src\%ORG_PATH% 2>nul
go get .\...
mklink /J "%GOPATH%\src\%REPO_PATH%" "%cd%" 2>nul

%GOROOT%\bin\go build -o bin\%GOARCH%\%MODULENAME%.exe %REPO_PATH%