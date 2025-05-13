# ✉️ IMAP Cleaning Tool

A lightweight Go utility for backing up, restoring, and selectively deleting emails from an IMAP server using simple filters.

---

## 🔧 Usage

```bash
imap-tool -h
```

```
  -allow-plain
        Allow PLAINTEXT auth over port 143 (insecure)
  -backup string
        Create a backup and exit
  -email string
        Email address to authenticate
  -field string
        Field to filter: from | to | subject (default "from")
  -imap string
        IMAP server address (host:port)
  -match string
        Text to match in the selected field
  -password string
        Password for the email account
  -restore string
        Restore from backup and exit
  -size
        Show message size (MB) in stats
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
