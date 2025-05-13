# IMAP Tool

A simple command-line utility to **backup**, **restore**, or **delete** emails from your IMAP mailbox.

---

## ✅ Key Use Cases

* 🧳 **Backup** all emails from an account to a `.tgz` file
* 🔁 **Restore** emails to a new mailbox
* 🧹 **Delete** mass emails (e.g. from a bug, spam, or bulk notifications)

---

## ⚡ Quick Install (One-liner per OS)

### 🐧 Linux

```bash
sudo sh -c 'curl -L https://files.zabiyaka.net/imap-tool/latest/linux/amd64/imap-tool -o /usr/local/bin/imap-tool && chmod +x /usr/local/bin/imap-tool' && imap-tool -h
```

---

### 🍏 macOS

```bash
sudo sh -c 'curl -L https://files.zabiyaka.net/imap-tool/latest/mac/amd64/imap-tool -o /usr/local/bin/imap-tool && chmod +x /usr/local/bin/imap-tool' && imap-tool -h
```

---

### 🧂 FreeBSD

```bash
sudo sh -c 'curl -L https://files.zabiyaka.net/imap-tool/latest/freebsd/amd64/imap-tool -o /usr/local/bin/imap-tool && chmod +x /usr/local/bin/imap-tool' && imap-tool -h
```

---

### 🧅 OpenBSD

```bash
sudo sh -c 'curl -L https://files.zabiyaka.net/imap-tool/latest/openbsd/amd64/imap-tool -o /usr/local/bin/imap-tool && chmod +x /usr/local/bin/imap-tool' && imap-tool -h
```

---

### 🪟 Windows (PowerShell)

```powershell
Invoke-WebRequest -Uri "https://files.zabiyaka.net/imap-tool/latest/windows/amd64/imap-tool.exe" -OutFile "$env:USERPROFILE\imap-tool.exe"; & "$env:USERPROFILE\imap-tool.exe" -h
```

> 🔗 For other platforms visit: [https://files.zabiyaka.net/imap-tool/latest](https://files.zabiyaka.net/imap-tool/latest)

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

## ♻️ Restore to Another Mailbox

```bash
imap-tool \
  -email user2@example.net \
  -password YOUR_PASSWORD \
  -imap imap.example.net:993 \
  -restore backup.tgz
```

---

## 🧹 Delete Emails by Sender

```bash
imap-tool \
  -email user@example.com \
  -password YOUR_PASSWORD \
  -imap imap.example.com:993 \
  -field from \
  -match spammer@example.net
```

You’ll be shown a list of senders with message counts. Confirm before deletion.

---

## 🧼 Delete Emails by Recipient

```bash
imap-tool \
  -email user@example.com \
  -password YOUR_PASSWORD \
  -imap imap.example.com:993 \
  -field to \
  -match support@example.org
```

---

## 🆘 Command Line Options

```bash
imap-tool -h

  -backup      Create backup and exit
  -restore     Restore from backup and exit
  -email       Email address
  -password    Email password
  -imap        IMAP server:port (e.g., imap.gmail.com:993)
  -field       from | to | subject (default: from)
  -match       Search text in selected field
  -size        Show message sizes in stats
```

---

## 💬 Example: Delete by Sender

```bash
imap-tool \
  -email bob@example.net \
  -password XXXXXX \
  -imap imap.example.net:993
```

```
FROM 21‑40 / 5877
┌────┬────────────────────────────────────┬────────┐
│  # │ FROM                               │  MSGS  │
├────┼────────────────────────────────────┼────────┤
│  1 │ alerts@notify.example.com          │    325 │
│  2 │ promo@store.example.com            │    303 │
│  3 │ noreply@security.example.net       │    287 │
└────┴────────────────────────────────────┴────────┘

num=del  n/p  q : 1
Delete ALL from "alerts@notify.example.com" (325)? (y/N):
```

---

## 💬 Example: Delete by Recipient

```bash
imap-tool \
  -email alice@example.com \
  -password XXXXXX \
  -imap imap.example.com:993 \
  -field to
```

```
TO 41‑60 / 1426
┌────┬────────────────────────────────────┬────────┐
│  # │ TO                                 │  MSGS  │
├────┼────────────────────────────────────┼────────┤
│  1 │ team@example.com                   │     78 │
│  2 │ dev@example.org                    │     77 │
│  3 │ support@example.org                │     61 │
└────┴────────────────────────────────────┴────────┘

num=del  n/p  q : 3
Delete ALL to "support@example.org" (61)? (y/N):
```
