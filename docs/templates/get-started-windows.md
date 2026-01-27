# SiloBang v{{VERSION}} - Windows Quick Start

## Starting the App

Double-click `silobang.exe` or run from Command Prompt:

```cmd
silobang.exe
```

Open http://localhost:2369 in your browser.

---

## Security Warning Bypass

Since this is an open-source project without a paid Microsoft certificate, Windows Defender SmartScreen will block the app.

**Fix:**
1. Click "More info"
2. Click "Run anyway"

Or add an exception in Windows Security settings.

---

## Windows Firewall

On first run, Windows may ask to allow network access. Click "Allow access" to enable the web dashboard.

---

## Verify Authenticity (Optional)

**Step 1:** Open PowerShell and run:
```powershell
Get-FileHash silobang.exe -Algorithm SHA256
```

**Step 2:** Compare with the hash in `SHA256SUMS.txt` from the release page.

**Step 3:** To fully verify, clone and build from source:
```bash
git clone {{REPO_URL}}.git
cd silobang
git checkout {{COMMIT_SHA}}
make build
```

Then compare:
```powershell
Get-FileHash silobang.exe -Algorithm SHA256
```

If the checksums match, the binary is authentic and unmodified.

---

## Version Info

```cmd
silobang.exe -version
```
