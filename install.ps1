# install.ps1 - download the latest GoDocHive (hiver) binary for windows.
#
#   irm https://raw.githubusercontent.com/intincrab/GoDocHive/main/install.ps1 | iex
#
# environment overrides:
#   $env:VERSION  tag to install (default: latest release, e.g. v0.2.0)
#   $env:BINDIR   install directory (default: %LOCALAPPDATA%\Programs\hiver)

$ErrorActionPreference = "Stop"

$repo = "intincrab/GoDocHive"
$project = "GoDocHive"

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }

$version = $env:VERSION
if (-not $version) {
    $release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
    $version = $release.tag_name
}
if (-not $version) { throw "could not determine the latest version" }
$verNoV = $version.TrimStart("v")

$asset = "${project}_${verNoV}_windows_${arch}.zip"
$url = "https://github.com/$repo/releases/download/$version/$asset"

$tmp = Join-Path $env:TEMP ("hiver_" + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Force $tmp | Out-Null
try {
    $zip = Join-Path $tmp $asset
    Write-Host "downloading $asset ..."
    Invoke-WebRequest $url -OutFile $zip
    Expand-Archive $zip -DestinationPath $tmp -Force

    $binDir = if ($env:BINDIR) { $env:BINDIR } else { Join-Path $env:LOCALAPPDATA "Programs\hiver" }
    New-Item -ItemType Directory -Force $binDir | Out-Null
    Copy-Item (Join-Path $tmp "hiver.exe") (Join-Path $binDir "hiver.exe") -Force

    Write-Host "installed hiver $version to $binDir\hiver.exe"

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$binDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$binDir", "User")
        Write-Host "added $binDir to your user PATH (open a new terminal to use 'hiver')"
    }
    Write-Host "run  hiver -path C:\path\to\your\docs  then open http://127.0.0.1:3030/search"
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
