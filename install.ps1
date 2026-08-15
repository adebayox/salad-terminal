$ErrorActionPreference = 'Stop'

$repo = if ($env:SALAD_TERMINAL_REPO) { $env:SALAD_TERMINAL_REPO } else { 'adebayox/salad-terminal' }
$release = if ($env:SALAD_TERMINAL_RELEASE) { $env:SALAD_TERMINAL_RELEASE } else { 'latest' }
$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq 'Arm64') { 'arm64' } else { 'amd64' }
$archive = "salad-windows-$arch.zip"

function Get-ReleaseTag {
  if ($release -ne 'latest') { return $release }
  $headers = @{ 'User-Agent' = 'salad-terminal-installer' }
  $latest = Invoke-RestMethod -Headers $headers -Uri "https://api.github.com/repos/$repo/releases/latest"
  if (-not $latest.tag_name) { throw "Could not resolve the latest immutable release for $repo" }
  return $latest.tag_name
}

$tag = Get-ReleaseTag
$base = if ($env:SALAD_TERMINAL_BASE_URL) { $env:SALAD_TERMINAL_BASE_URL } else { "https://github.com/$repo/releases/download/$tag" }
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("salad-terminal-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  $archivePath = Join-Path $tmp $archive
  $checksumsPath = Join-Path $tmp 'SHA256SUMS'
  Invoke-WebRequest -UseBasicParsing -Uri "$base/$archive" -OutFile $archivePath
  Invoke-WebRequest -UseBasicParsing -Uri "$base/SHA256SUMS" -OutFile $checksumsPath

  $expected = (Get-Content $checksumsPath | Where-Object { $_ -match "\s$([regex]::Escape($archive))$" } | Select-Object -First 1) -split '\s+' | Select-Object -First 1
  if (-not $expected) { throw "SHA256SUMS does not contain $archive" }
  $actual = (Get-FileHash -Algorithm SHA256 -Path $archivePath).Hash.ToLowerInvariant()
  if ($actual -ne $expected.ToLowerInvariant()) { throw "Checksum verification failed for $archive" }

  $extract = Join-Path $tmp 'extract'
  Expand-Archive -Path $archivePath -DestinationPath $extract -Force
  $binary = Join-Path $extract 'salad.exe'
  if (-not (Test-Path $binary)) { throw 'Release archive did not contain salad.exe' }

  $binDir = if ($env:SALAD_BIN_DIR) { $env:SALAD_BIN_DIR } else { Join-Path $env:LOCALAPPDATA 'Salad\bin' }
  New-Item -ItemType Directory -Force -Path $binDir | Out-Null
  $destination = Join-Path $binDir 'salad.exe'
  $staged = Join-Path $binDir ('.salad.exe.' + [guid]::NewGuid().ToString('N') + '.new')
  $backup = Join-Path $binDir 'salad.exe.previous'
  try {
    # Stage beside the installed binary, then replace it atomically. A failed
    # update leaves the existing CLI usable and preserves the previous binary
    # for an operator rollback instead of leaving a half-written executable.
    Copy-Item -Force $binary $staged
    if (Test-Path $destination) {
      [System.IO.File]::Replace($staged, $destination, $backup, $true)
    } else {
      Move-Item -Force $staged $destination
    }
  } catch {
    if ((Test-Path $backup) -and -not (Test-Path $destination)) {
      Copy-Item -Force $backup $destination
    }
    throw
  } finally {
    Remove-Item -Force $staged -ErrorAction SilentlyContinue
  }
  $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  if (-not $userPath) { $userPath = '' }
  if (($userPath -split ';') -notcontains $binDir) {
    [Environment]::SetEnvironmentVariable('Path', (($userPath.TrimEnd(';') + ';' + $binDir).Trim(';')), 'User')
  }
  Write-Host "Installed: $binDir\salad.exe"
  Write-Host 'Open a new PowerShell window, then run: salad'
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
