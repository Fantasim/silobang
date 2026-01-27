# SiloBang v{{VERSION}} - macOS Quick Start

## Starting the App

```bash
./silobang
```

Open http://localhost:2369 in your browser.

---

## Security Warning Bypass

Since this is an open-source project without a paid Apple Developer certificate, macOS will block the app.

**Fix:** Remove quarantine attribute:
```bash
xattr -d com.apple.quarantine silobang
```

Or right-click the binary, select "Open", then click "Open" again in the dialog.

---

## Verify Authenticity (Optional)

**Step 1:** Run SHA256 checksum:
```bash
shasum -a 256 silobang
```

**Step 2:** Compare with the hash in `SHA256SUMS.txt` from the release page.

**Step 3:** To fully verify, clone and build from source:
```bash
git clone {{REPO_URL}}.git
cd silobang
git checkout {{COMMIT_SHA}}
make build
shasum -a 256 silobang
```

If the checksums match, the binary is authentic and unmodified.

---

## Version Info

```bash
./silobang -version
```
