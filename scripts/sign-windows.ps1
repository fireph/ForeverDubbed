param(
    [string] $File,
    [switch] $CheckOnly
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

foreach ($name in @('FDB_SIGNING_ENDPOINT', 'FDB_SIGNING_ACCOUNT', 'FDB_SIGNING_PROFILE')) {
    if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($name))) {
        throw "Missing $name. See docs/windows-signing.md."
    }
}
# Keep this version in sync with the workflow's Install-Module step.
Import-Module ArtifactSigning -RequiredVersion 0.1.8 -ErrorAction Stop
if ($CheckOnly) { return }
if ([string]::IsNullOrWhiteSpace($File)) { throw 'A file to sign is required.' }
$target = (Resolve-Path -LiteralPath $File).Path

# azure/login establishes the CLI session using GitHub OIDC. Local builds can
# use az login. No long-lived client secret or exported signing key is needed.
$parameters = @{
    Endpoint = $env:FDB_SIGNING_ENDPOINT
    CodeSigningAccountName = $env:FDB_SIGNING_ACCOUNT
    CertificateProfileName = $env:FDB_SIGNING_PROFILE
    Files = $target
    FileDigest = 'SHA256'
    TimestampRfc3161 = 'http://timestamp.acs.microsoft.com'
    TimestampDigest = 'SHA256'
    ExcludeEnvironmentCredential = $true
    ExcludeWorkloadIdentityCredential = $true
    ExcludeManagedIdentityCredential = $true
    ExcludeSharedTokenCacheCredential = $true
    ExcludeVisualStudioCredential = $true
    ExcludeVisualStudioCodeCredential = $true
    ExcludeAzureCliCredential = $false
    ExcludeAzurePowerShellCredential = $true
    ExcludeAzureDeveloperCliCredential = $true
    ExcludeInteractiveBrowserCredential = $true
}
Invoke-ArtifactSigning @parameters

$signature = Get-AuthenticodeSignature -LiteralPath $target
if ($signature.Status -ne 'Valid' -or $null -eq $signature.SignerCertificate) {
    throw "Signature verification failed for ${target}: $($signature.Status) $($signature.StatusMessage)"
}
if ($null -eq $signature.TimeStamperCertificate) {
    throw "Missing trusted timestamp on $target"
}
Write-Host "Verified signed and timestamped file: $target"
