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

  -backup         Create backup and exit
  -restore        Restore from backup and exit
  -email          Email address
  -password       Email password
  -imap           IMAP server:port
  -field          from | to | subject (default: from)
  -match          Text to search in field
  -size           Show message size in stats
```

---

## 🗂 Backup Example

```bash
imap-tool \
  -email alice@example.com \
  -password XXXXXX \
  -imap imap.example.com:993 \
  -backup backup-alice.tgz
```

```
⚠️  Legacy TLS
🔄 Backing up → backup-alice.tgz
📦 Folders: 3 | Messages: 429
✅ Backup completed successfully
```

---

## ♻️ Restore Example

```bash
imap-tool \
  -email bob@example.net \
  -password XXXXXX \
  -imap imap.example.net:993 \
  -restore backup-alice.tgz
```

```
⚠️  Legacy TLS
🔄 Restoring ← backup-alice.tgz
✅ Restore completed successfully
```

---

## 🧹 Delete Emails by Sender

Delete all emails **from** a specific sender, e.g., `newsletter@updates.com`:

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
│  4 │ updates@blog.example.org           │    264 │
│  5 │ newsletter@updates.com             │    226 │
└────┴────────────────────────────────────┴────────┘

num=del  n/p  q : 5
Delete ALL from "newsletter@updates.com" (226)? (y/N):
```

---

## 🧹 Delete Emails by Recipient

Delete all emails **sent to** a specific address, e.g., `support@example.org`:

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
│  3 │ office@example.net                 │     76 │
│  4 │ manager@example.com                │     76 │
│  5 │ support@example.org                │     61 │
└────┴────────────────────────────────────┴────────┘

num=del  n/p  q : 5
Delete ALL to "support@example.org" (61)? (y/N):
```

