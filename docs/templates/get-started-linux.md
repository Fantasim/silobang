# SiloBang v{{VERSION}} - Linux Quick Start

## Starting the App

```bash
chmod +x silobang
./silobang
```

Open http://localhost:2369 in your browser.

---

## Security Warning Bypass

Since this is an open-source project without a paid developer certificate, Linux may block execution.

**Fix:** Make the binary executable:
```bash
chmod +x silobang
```

---

## Verify Authenticity (Optional)

**Step 1:** Run SHA256 checksum:
```bash
sha256sum silobang
```

**Step 2:** Compare with the hash in `SHA256SUMS.txt` from the release page.

**Step 3:** To fully verify, clone and build from source:
```bash
git clone {{REPO_URL}}.git
cd silobang
git checkout {{COMMIT_SHA}}
make build
sha256sum silobang
```

If the checksums match, the binary is authentic and unmodified.

---

## Version Info

```bash
./silobang -version
```

---

## Systemd Service (Optional)

Create `/etc/systemd/system/silobang.service`:
```ini
[Unit]
Description=SiloBang Asset Server
After=network.target

[Service]
Type=simple
ExecStart=/opt/silobang/silobang
WorkingDirectory=/opt/silobang
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Then:
```bash
sudo systemctl enable silobang
sudo systemctl start silobang
```
