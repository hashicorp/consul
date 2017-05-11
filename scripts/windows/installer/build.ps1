[CmdletBinding()]
Param (
  [Parameter(Mandatory=$true)]
  [String] $Version,
  [Parameter(Mandatory=$false)]
  [ValidateSet("amd64","386")]
  [String] $Arch = "amd64"
)
$ErrorActionPreference = "Stop"

Add-Type -AssemblyName System.IO.Compression.FileSystem
function Expand-ZipFile {
  Param (
    $File,
    $DestinationPath
  )
  $fullFilePath = Resolve-Path $File # Full paths are required, so make sure it is expanded
  $fullDestinationPath = Resolve-Path $DestinationPath

  [System.IO.Compression.ZipFile]::ExtractToDirectory($fullFilePath, $fullDestinationPath)
}

$wc = New-Object System.Net.WebClient
function Get-FileIfNotExists {
  Param (
    $Url,
    $Destination
  )
  if(-not (Test-Path $Destination)) {
    Write-Verbose "Downloading $Url"
    $wc.DownloadFile($Url, $Destination)
  }
  else {
    Write-Verbose "${Destination} already exists. Skipping."
  }
}

$sourceDir = mkdir -Force Source
mkdir -Force Work,Output | Out-Null

Write-Verbose "Downloading files"
Get-FileIfNotExists "https://releases.hashicorp.com/consul/${Version}/consul_${Version}_windows_${Arch}.zip" "${sourceDir}\consul_${Version}_windows_${Arch}.zip"
Get-FileIfNotExists "http://repo.jenkins-ci.org/releases/com/sun/winsw/winsw/1.18/winsw-1.18-bin.exe" "$sourceDir\winsw.exe"
# Somewhat obscure url, points to WiX 3.10 binary release
Get-FileIfNotExists "http://download-codeplex.sec.s-msft.com/Download/Release?ProjectName=wix&DownloadId=1504735&FileTime=130906491728530000&Build=21031" "$sourceDir\wix-binaries.zip"

Write-Verbose "Unpacking"
Expand-ZipFile -File "${sourceDir}\consul_${Version}_windows_${Arch}.zip" -DestinationPath "Work\"
Move-Item -Force "Work\consul.exe" "Work\consul-${Version}-${Arch}.exe"

Copy-Item -Force "${sourceDir}\winsw.exe" Work\

mkdir -Force WiX | Out-Null
Remove-Item -Recurse -Force WiX\* # Below function can't deal with existing files, sadly. PS5 has Expand-Archive, but it is not widely deployed :(
Expand-ZipFile -File "${sourceDir}\wix-binaries.zip" -DestinationPath WiX

Write-Verbose "Creating consul-${Version}-${Arch}.msi"
$wixArch = @{"amd64"="x64"; "386"="x86"}[$Arch]
WiX\candle.exe -nologo -arch $wixArch -ext WixFirewallExtension -out Work\consul.wixobj -dVersion="$Version" consul.wxs
WiX\light.exe -nologo -spdb -ext WixFirewallExtension -out "Output\consul-${Version}-${Arch}.msi" Work\consul.wixobj

Write-Verbose "Done!"