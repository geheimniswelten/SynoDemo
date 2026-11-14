@echo off
setlocal DisableDelayedExpansion
rem Sonderfall SynoDemo: Die Demo-Releases und Demo-Tags werden OHNE Rueckfrage ersetzt.
rem Alte Demo-Version v1.1.0 wird ebenfalls entfernt. Andere Releases bleiben erhalten.
cd /d "%~dp0"
if not exist "%~dp0RELEASE.config.cmd" (echo RELEASE.config.cmd fehlt. & exit /b 1)
call "%~dp0RELEASE.config.cmd"
if /i not "%REPO%"=="geheimniswelten/SynoDemo" (echo Falsches GitHub-Repository in RELEASE.config.cmd. & exit /b 1)
for /f "delims=" %%R in ('git remote get-url "%REMOTE%" 2^>nul') do set "REMOTE_URL=%%R"
if /i "%REMOTE_URL%"=="git@github.com:geheimniswelten/SynoDemo.git" goto :remote_ok
if /i "%REMOTE_URL%"=="https://github.com/geheimniswelten/SynoDemo.git" goto :remote_ok
if /i "%REMOTE_URL%"=="ssh://git@github.com/geheimniswelten/SynoDemo.git" goto :remote_ok
echo Der Git-Remote zeigt nicht auf das erwartete SynoDemo-Repository.
exit /b 1
:remote_ok

rem Vor dem Loeschen muessen beide vollstaendigen Builds samt Pruefsummen vorliegen.
for %%V in (1.0.0 1.2.0) do for %%A in (alpine alpine4k rtd1296 x86_64) do call :checkfile "%%V" "%%A" || exit /b 1
where gh >nul 2>&1
if errorlevel 1 if exist "%ProgramFiles%\GitHub CLI\gh.exe" set "PATH=%ProgramFiles%\GitHub CLI;%PATH%"
where gh >nul 2>&1 || (echo GitHub CLI fehlt. & exit /b 1)
gh auth status >nul 2>&1 || (echo GitHub CLI bitte zuerst mit gh auth login --web anmelden. & exit /b 1)
git rev-parse --verify HEAD >nul 2>&1 || (echo Git-Commit fehlt. & exit /b 1)
git remote get-url "%REMOTE%" >nul 2>&1 || (echo Git-Remote fehlt. & exit /b 1)
git fetch "%REMOTE%" "refs/tags/*:refs/tags/*" || exit /b 1

rem Die Liste wird vorab geladen; bei Netzwerk- oder API-Fehlern nichts loeschen.
set "RELEASE_LIST=%TEMP%\synodemo-releases-%RANDOM%-%RANDOM%.txt"
gh release list --repo "%REPO%" --limit 1000 --json tagName > "%RELEASE_LIST%" || goto :failed
powershell.exe -NoProfile -Command "$ErrorActionPreference='Stop'; $null=ConvertFrom-Json -InputObject ([IO.File]::ReadAllText($env:RELEASE_LIST))" || goto :failed

echo Ersetze ausschliesslich v1.0.0, v1.1.0 und v1.2.0 im Repository %REPO%.
for %%T in (v1.0.0 v1.1.0 v1.2.0) do call :remove_demo_release_and_tag %%T || goto :failed

del "%RELEASE_LIST%" >nul 2>&1
set "RELEASE_NONINTERACTIVE=1"
set "RELEASE_VERSION=1.0.0"
call "%~dp0RELEASE.cmd" || exit /b 1
set "RELEASE_VERSION=1.2.0"
call "%~dp0RELEASE.cmd" || exit /b 1
exit /b 0

:checkfile
set "CHECK_FILE=dist\h5uSynoDemo-%~1-0000-%~2.spk"
if not exist "%CHECK_FILE%" (echo Fehlt: %CHECK_FILE% & exit /b 1)
if not exist "%CHECK_FILE%.sha256" (echo Fehlt: %CHECK_FILE%.sha256 & exit /b 1)
powershell.exe -NoProfile -Command "$ErrorActionPreference='Stop'; $f=$env:CHECK_FILE; $x=([IO.File]::ReadAllText($f+'.sha256').Trim() -split '\s+')[0]; $a=[Security.Cryptography.SHA256]::Create(); $s=[IO.File]::OpenRead($f); try { $v=([BitConverter]::ToString($a.ComputeHash($s))).Replace('-','') } finally { $s.Dispose(); $a.Dispose() }; if ($v -ine $x) { exit 1 }" || (echo Pruefsumme fehlerhaft: %CHECK_FILE% & exit /b 1)
exit /b 0

:remove_demo_release_and_tag
set "CHECK_TAG=%~1"
powershell.exe -NoProfile -Command "try { $ErrorActionPreference='Stop'; $items=ConvertFrom-Json -InputObject ([IO.File]::ReadAllText($env:RELEASE_LIST)); if (@($items | Where-Object { $_.tagName -ceq $env:CHECK_TAG }).Count -gt 0) { exit 0 } else { exit 1 } } catch { exit 2 }"
if errorlevel 2 exit /b 1
if errorlevel 1 goto :remove_remote_tag
echo Altes Demo-Release %~1 entfernen ...
gh release delete "%~1" --repo "%REPO%" --yes || exit /b 1
:remove_remote_tag
git ls-remote --exit-code --tags "%REMOTE%" "refs/tags/%~1" >nul
if errorlevel 128 exit /b 1
if errorlevel 2 goto :remove_local_tag
if errorlevel 1 exit /b 1
git push "%REMOTE%" ":refs/tags/%~1" || exit /b 1
:remove_local_tag
git show-ref --verify --quiet "refs/tags/%~1"
if errorlevel 1 exit /b 0
git tag --delete "%~1" || exit /b 1
exit /b 0

:failed
del "%RELEASE_LIST%" >nul 2>&1
echo Fehler. Demo-Releases oder Tags koennen bereits teilweise geaendert sein.
exit /b 1
