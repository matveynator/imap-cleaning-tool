# IMAP Tool

A simple command-line utility to **backup**, **restore**, or **delete** emails from your IMAP mailbox.

## ✅ Key Use Cases

* 🧳 Backup all your emails from one account
* 🔁 Restore emails into another mailbox
* 🧹 Mass delete emails (e.g. from a buggy script or spam burst)

---

## ⚡ Quick Install

### Linux / macOS / BSD

```bash
sudo curl -L https://files.zabiyaka.net/imap-tool/latest/linux/amd64/imap-tool -o /usr/local/bin/imap-tool
sudo chmod +x /usr/local/bin/imap-tool
imap-tool -h
```

> For macOS, FreeBSD, OpenBSD: just change `linux` to your OS name in the URL.

### Windows (PowerShell)

```powershell
Invoke-WebRequest -Uri "https://files.zabiyaka.net/imap-tool/latest/windows/amd64/imap-tool.exe" -OutFile "$env:USERPROFILE\imap-tool.exe"
& "$env:USERPROFILE\imap-tool.exe" -h
```

---

## 📦 Backup All Emails

```bash
imap-tool \
  -email user1@example.com \
  -password YOUR_PASSWORD \
  -imap imap.example.com:993 \
  -backup backup.tgz
```

---

## ♻️ Restore Emails to Another Account

```bash
imap-tool \
  -email user2@example.net \
  -password YOUR_PASSWORD \
  -imap imap.example.net:993 \
  -restore backup.tgz
```

---

## 🧹 Delete Spam or Buggy Emails by Sender

```bash
imap-tool \
  -email user@example.com \
  -password YOUR_PASSWORD \
  -imap imap.example.com:993 \
  -field from \
  -match spammer@example.net
```

You’ll be shown a list and asked for confirmation before deleting.

---

## 🧼 Delete Emails by Recipient

```bash
imap-tool \
  -email user@example.com \
  -password YOUR_PASSWORD \
  -imap imap.example.com:993 \
  -field to \
  -match support@example.com
```

---

## 🆘 Help / Options

```bash
imap-tool -h
```

```
  -backup         Create backup and exit
  -restore        Restore from backup and exit
  -email          Email address
  -password       Email password
  -imap           IMAP server:port
  -field          from | to | subject (default: from)
  -match          Text to search in field
  -size           Show message size in stats
```
