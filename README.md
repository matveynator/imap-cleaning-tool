# Imap cleaing tool

```
go run imap-cleaing-tool.go -h  
  -allow-plain
    	Allow PLAINTEXT fallback on port 143
  -email string
    	Email address
  -field string
    	Header to use: from | to | subject (default "from")
  -imap string
    	IMAP host:port (guessed if empty)
  -match string
    	Text to match (deletes interactively)
  -password string
    	Password
  -size
    	Add MB column (stats mode only, slower)
```


