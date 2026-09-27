# Windows release signing

The Windows job builds on `windows-2025` with MSYS2 UCRT64 (MinGW-w64 GCC/G++) and NSIS. Non-PR builds require Azure Artifact Signing; missing configuration or a signing/verification error fails the build before artifact upload or release publishing. Pull requests build unsigned test artifacts without logging into Azure.

The packager signs and verifies `foreverdubbed.exe` and `foreverdubbed-updater.exe` before generating the update manifest and portable ZIP. NSIS signs its embedded uninstaller through `!uninstfinalize`, then the packager signs the finished installer. Every signature uses SHA-256 and Microsoft's RFC 3161 timestamp service, and verification requires both a valid signature and a timestamp. Third-party DLLs retain their original signatures.

## Azure setup

1. Create an **Artifact Signing** account on the Basic tier in a supported region. Under **Access control (IAM) → Add → Add role assignment**, assign **Artifact Signing Identity Verifier** to your own signed-in Azure user. Owner/Contributor alone does not include this permission. Your user also needs at least Reader access at subscription scope. Allow a few minutes for the role to take effect, then complete identity validation and create a **Public Trust** certificate profile (not Public Trust Test or Private Trust). Record the account name, profile name, and regional signing endpoint shown by Azure.
2. In Microsoft Entra ID, create an **App registration** for the GitHub workflow. Record its **Application (client) ID** and **Directory (tenant) ID**, plus your Azure **Subscription ID**.
3. Under the app registration's **Certificates & secrets → Federated credentials**, add a GitHub Actions credential for your repository with entity type **Environment**, named `windows-signing`. Its values should be:
   - Issuer: `https://token.actions.githubusercontent.com`
   - Subject for this repository: `repo:fireph@443370/ForeverDubbed@1380626391:environment:windows-signing`
   - GitHub owner/org ID: `443370`; repository ID: `1380626391`. Enter these if the Azure form requests numeric IDs.
   - Audience: `api://AzureADTokenExchange`
4. On the signing account's **Access control (IAM)** page, grant that application's service principal the **Artifact Signing Certificate Profile Signer** role. Older Azure UI may label this **Trusted Signing Certificate Profile Signer**. For tighter scope, use Azure CLI to assign the role on the specific certificate profile as shown in [Microsoft's role assignment guide](https://learn.microsoft.com/en-us/azure/artifact-signing/tutorial-assign-roles). Subscription Contributor by itself does not grant signing permission.

This repository uses GitHub's immutable OIDC subject format, so the numeric IDs are required in the subject above. For a different repository, check `gh api repos/OWNER/REPOSITORY/actions/oidc/customization/sub` and append `:environment:windows-signing` to its `sub_claim_prefix`. See [Microsoft's immutable subject guide](https://learn.microsoft.com/en-us/entra/workload-id/workload-identities-github-immutable-subjects).

No client secret, PFX file, or private signing key needs to be stored in GitHub. GitHub OIDC authenticates the workflow, and Azure retains the signing key.

## GitHub settings

In the repository, open **Settings → Environments**, create `windows-signing`, and add these **environment variables**:

| Variable | Value |
| --- | --- |
| `AZURE_CLIENT_ID` | Application (client) ID from the app registration |
| `AZURE_TENANT_ID` | Directory (tenant) ID |
| `AZURE_SUBSCRIPTION_ID` | Subscription containing the signing account |
| `FDB_SIGNING_ENDPOINT` | Regional endpoint, for example `https://eus.codesigning.azure.net/`—use your account's region |
| `FDB_SIGNING_ACCOUNT` | Artifact Signing account name, not its resource ID |
| `FDB_SIGNING_PROFILE` | Public Trust certificate profile name |

These values are identifiers, not passwords. Restrict the environment's deployment branches/tags to `main` and release tags matching `v*`. A required reviewer is optional. PR jobs use the separate `windows-testing` environment, which must have no Azure federated credential or signing configuration.

Once configured, run **Actions → Build and release → Run workflow** on `main`. This builds signed downloadable artifacts without publishing a GitHub release. Check for `Verified signed and timestamped file` messages for the app, updater, uninstaller, and installer. A version-tag push publishes only after the Windows and macOS jobs succeed; release checksums are generated from the final signed downloads.

Until these settings and the Azure identity/profile are ready, non-PR Windows builds intentionally fail with a configuration or signing error. There is no automatic unsigned fallback.

## Local builds

Unsigned builds and Linux cross-compilation still work with the existing commands. To sign locally, use Windows with PowerShell 7, Azure CLI, .NET 8, Go, MinGW-w64, CMake, and NSIS 3.08+ installed. Sign in with `az login` using an identity with the signing role, set the three `FDB_SIGNING_*` environment variables above, and install the pinned Microsoft module:

```powershell
Install-Module ArtifactSigning -RequiredVersion 0.1.8 -Scope CurrentUser -Force -Repository PSGallery
go run ./tools/build -target windows -arch amd64 -native-dir .runtime/native-windows -windows-installer -windows-sign
```

Signed files must not be modified afterward. Timestamped signatures remain verifiable after the short-lived signing certificate expires; SmartScreen reputation is separate and new signed releases can still show warnings.

References: [Azure setup](https://learn.microsoft.com/en-us/azure/artifact-signing/quickstart), [Microsoft signing integration and OIDC example](https://github.com/Azure/artifact-signing-action), [NSIS uninstaller signing](https://nsis.sourceforge.io/Docs/Chapter5.html#uninstfinalize), [SmartScreen reputation](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation).
