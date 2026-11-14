#requires -Version 5.1
[CmdletBinding()]
param(
    [string]$Python = $env:SYNODEMO_PYTHON,
    [string]$Go = $env:SYNODEMO_GO,
    [ValidateSet("1.0.0-0000", "1.2.0-0000", "both")]
    [string]$Version = "both",
    [ValidateSet("all", "x86_64", "rtd1296", "alpine", "alpine4k")]
    [string]$Architecture = "all"
)
$ErrorActionPreference = 'Stop'

function Find-Tool {
    param([string]$Override, [string[]]$Candidates, [string]$Portable, [string]$Label)
    if ($Override) { $Candidates = @($Override) }
    else { $Candidates += $Portable }
    foreach ($name in $Candidates) {
        $command = Get-Command -Name $name -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($command -and $command.Source -notlike '*\Microsoft\WindowsApps\*') {
            return $command.Source
        }
    }
    throw "No $Label executable found. Install it or supply its path."
}

try {
    $portableDir = Join-Path $PSScriptRoot '..\SynoWake\work\toolchains'
    $pythonExecutable = Find-Tool $Python @('python', 'python3', 'py') (Join-Path $portableDir 'python-3.13.12-amd64\python.exe') 'Python 3.10+'
    $goExecutable = Find-Tool $Go @('go') (Join-Path $portableDir 'go-1.27.1-amd64\go\bin\go.exe') 'Go'
    $pythonArguments = @()
    if ([System.IO.Path]::GetFileNameWithoutExtension($pythonExecutable) -eq 'py') {
        $pythonArguments += '-3'
    }
    $pythonArguments += @((Join-Path $PSScriptRoot 'scripts\build.py'), '--go', $goExecutable)
    $versions = if ($Version -eq "both") { @("1.0.0-0000", "1.2.0-0000") } else { @($Version) }
    $architectures = if ($Architecture -eq "all") { @("x86_64", "rtd1296", "alpine", "alpine4k") } else { @($Architecture) }
    foreach ($item in $versions) {
        foreach ($arch in $architectures) {
            & $pythonExecutable @pythonArguments --version $item --arch $arch
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        }
    }
    exit 0
}
catch {
    [Console]::Error.WriteLine("BUILD failed: {0}", $_.Exception.Message)
    exit 1
}
