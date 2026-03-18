param(
    [Parameter(Mandatory=$true)]
    [string]$RepoUrl,
    [Parameter(Mandatory=$true)]
    [string]$Token
)

$RunnerVersion = "2.332.0"
$RunnerDir = "C:\actions-runner"

if (-not (Test-Path $RunnerDir)) {
    New-Item -ItemType Directory -Path $RunnerDir | Out-Null
}
Set-Location $RunnerDir

$ZipFile = "actions-runner-win-x64-$RunnerVersion.zip"
$DownloadUrl = "https://github.com/actions/runner/releases/download/v$RunnerVersion/$ZipFile"

if (-not (Test-Path $ZipFile)) {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipFile -UseBasicParsing
}

Add-Type -AssemblyName System.IO.Compression.FileSystem
[System.IO.Compression.ZipFile]::ExtractToDirectory("$PWD\$ZipFile", "$PWD")

.\config.cmd --url $RepoUrl --token $Token --unattended