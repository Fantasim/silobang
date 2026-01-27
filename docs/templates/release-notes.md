<details><summary>See patch notes</summary>

<br />

{{CHANGELOG}}

<br />
</details>

---

## Download & Installation

Choose the archive for your platform:

| Platform | Architecture | Download |
|----------|-------------|----------|
| Linux | x86_64 | [silobang-v{{VERSION}}-linux-amd64.tar.gz]({{REPO_URL}}/releases/download/v{{VERSION}}/silobang-v{{VERSION}}-linux-amd64.tar.gz) |
| Linux | ARM64 | [silobang-v{{VERSION}}-linux-arm64.tar.gz]({{REPO_URL}}/releases/download/v{{VERSION}}/silobang-v{{VERSION}}-linux-arm64.tar.gz) |
| macOS | Intel | [silobang-v{{VERSION}}-macos-amd64.tar.gz]({{REPO_URL}}/releases/download/v{{VERSION}}/silobang-v{{VERSION}}-macos-amd64.tar.gz) |
| macOS | Apple Silicon | [silobang-v{{VERSION}}-macos-arm64.tar.gz]({{REPO_URL}}/releases/download/v{{VERSION}}/silobang-v{{VERSION}}-macos-arm64.tar.gz) |
| Windows | x86_64 | [silobang-v{{VERSION}}-windows-amd64.zip]({{REPO_URL}}/releases/download/v{{VERSION}}/silobang-v{{VERSION}}-windows-amd64.zip) |

---

## Quick Start

<details><summary><b>Linux</b></summary>

```bash
tar -xzf silobang-v{{VERSION}}-linux-*.tar.gz
cd silobang-v{{VERSION}}-linux-*/
chmod +x silobang
./silobang
```

Open http://localhost:2369 in your browser.

**Security note:** Since this is an open-source project without a paid developer certificate, Linux may block execution. Fix with `chmod +x silobang`.

</details>

<details><summary><b>macOS</b></summary>

```bash
tar -xzf silobang-v{{VERSION}}-macos-*.tar.gz
cd silobang-v{{VERSION}}-macos-*/
xattr -d com.apple.quarantine silobang
./silobang
```

Open http://localhost:2369 in your browser.

**Security note:** Since this is an open-source project without a paid Apple Developer certificate, macOS will block the app. Remove quarantine with `xattr -d com.apple.quarantine silobang`, or right-click the binary and select "Open".

</details>

<details><summary><b>Windows</b></summary>

1. Extract the ZIP archive
2. Open Command Prompt or PowerShell in the extracted folder
3. Run:
```cmd
silobang.exe
```
4. Open http://localhost:2369 in your browser.

**Security note:** Windows Defender SmartScreen may block the app. Click "More info" then "Run anyway".

</details>

---

## Build Verification

<details><summary>See build verification process</summary>

<br />

**Commit:** [`{{COMMIT_SHORT}}`]({{REPO_URL}}/commit/{{COMMIT_SHA}})

You can verify these binaries are authentic by reproducing the build locally.

**Step 1:** Clone and checkout the release commit
```bash
git clone {{REPO_URL}}.git
cd silobang
git checkout {{COMMIT_SHA}}
```

**Step 2:** Build locally
```bash
make build
```

**Step 3:** Compare SHA256 checksums

On Linux:
```bash
sha256sum silobang
```

On macOS:
```bash
shasum -a 256 silobang
```

On Windows (PowerShell):
```powershell
Get-FileHash silobang.exe -Algorithm SHA256
```

If the checksums match, the binary is authentic and unmodified.

</details>

### SHA256 Checksums
```
{{CHECKSUMS}}
```
