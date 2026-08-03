@echo off
REM === ModuForge Build Script ===
REM Usage: build.bat [dev|release|clean|size]

setlocal
set APP_NAME=moduforge
set CMD=%1
if "%CMD%"=="" set CMD=dev

if "%CMD%"=="dev" goto :dev
if "%CMD%"=="release" goto :release
if "%CMD%"=="clean" goto :clean
if "%CMD%"=="size" goto :size
echo Usage: %~nx0 [dev^|release^|clean^|size]
goto :end

:dev
echo Building %APP_NAME% (dev)...
cd /d "%~dp0backend\cmd\moduforge"
go build -o "..\..\bin\%APP_NAME%.exe" .
echo Done: bin\%APP_NAME%.exe
goto :end

:release
echo Building %APP_NAME% (release, stripped)...
cd /d "%~dp0backend\cmd\moduforge"
go build -ldflags="-s -w" -trimpath -o "..\..\bin\%APP_NAME%.exe" .
echo Done: bin\%APP_NAME%.exe
for %%f in ("..\bin\%APP_NAME%.exe") do echo Size: %%~zf bytes
goto :end

:clean
echo Cleaning build cache...
cd /d "%~dp0backend"
go clean -cache -testcache
if exist "%~dp0bin\%APP_NAME%.exe" del /q "%~dp0bin\%APP_NAME%.exe"
echo Cleaned.
goto :end

:size
echo === Backend Binary ===
for %%f in ("%~dp0bin\*.exe") do echo %%f: %%~zf bytes
echo === Frontend Dist ===
dir /s /b "%~dp0frontend\dist\*" 2>nul | find /c /v "" && echo files
goto :end

:end
endlocal
