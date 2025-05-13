// imap-cleaning-tool.go  •  2025‑05‑13
//
// Fast IMAP scanner / cleaner with real‑time progress.
//  • -match "text"         delete everything whose FIELD contains text
//  • -field from|to|subject  grouping / filtering field   (default: from)
//  • -size                 include MB column in *stats* mode (size always on in -match)
//  • if -match omitted     statistics mode (Top‑N, interactive delete)
//  • TLS fallback: modern → legacy → (optional) plain  (use -allow-plain on port 143)

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/mail"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

/* ───── Flags ───── */

var (
	email      = flag.String("email", "", "Email address")
	password   = flag.String("password", "", "Password")
	imapHost   = flag.String("imap", "", "IMAP host:port (guessed if empty)")
	match      = flag.String("match", "", "Text to match (deletes interactively)")
	field      = flag.String("field", "from", "Header to use: from | to | subject")
	sizeFlag   = flag.Bool("size", false, "Add MB column (stats mode only, slower)")
	allowPlain = flag.Bool("allow-plain", false, "Allow PLAINTEXT fallback on port 143")
	pageSize   = 20
)

/* ───── TLS helper ───── */

func dialSmart(addr string) (*client.Client, error) {
	host, port, _ := net.SplitHostPort(addr)
	mod := &tls.Config{ServerName: host, InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}
	leg := &tls.Config{ServerName: host, InsecureSkipVerify: true, MinVersion: tls.VersionTLS10,
		CipherSuites: []uint16{tls.TLS_RSA_WITH_RC4_128_SHA, tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA, tls.TLS_RSA_WITH_AES_256_CBC_SHA}}

	connect := func(cfg *tls.Config) (*client.Client, error) {
		switch port {
		case "993":
			return client.DialTLS(addr, cfg)
		case "143":
			c, err := client.Dial(addr)
			if err != nil {
				return nil, err
			}
			if err = c.StartTLS(cfg); err != nil {
				c.Logout()
				return nil, err
			}
			return c, nil
		default:
			return nil, fmt.Errorf("unsupported port %s", port)
		}
	}
	if c, err := connect(mod); err == nil {
		fmt.Println("✅  Modern TLS")
		return c, nil
	}
	if c, err := connect(leg); err == nil {
		fmt.Println("⚠️  Legacy TLS")
		return c, nil
	}
	if port == "143" && *allowPlain {
		fmt.Println("⚠️  Plain IMAP (NO encryption)")
		return client.Dial(addr)
	}
	return nil, fmt.Errorf("TLS negotiation failed")
}

func guessServer(addr string) (string, error) {
	a, _ := mail.ParseAddress(addr)
	domain := strings.Split(a.Address, "@")[1]
	for _, p := range []string{"imap.", "mail.", ""} {
		h := p + domain + ":993"
		if _, err := tls.Dial("tcp", h, &tls.Config{InsecureSkipVerify: true}); err == nil {
			return h, nil
		}
	}
	return domain + ":143", nil
}

/* ───── Data ───── */

type bucket struct {
	Key       string
	Count     int
	Bytes     int64
	ByFolder  map[string][]uint32
}

func (b *bucket) add(folder string, id uint32, size int64) {
	b.Count++
	b.Bytes += size
	b.ByFolder[folder] = append(b.ByFolder[folder], id)
}

/* ───── Utility helpers ───── */

func classify(m *imap.Message, fld string) string {
	addr := func(l []*imap.Address) string {
		if len(l) == 0 {
			return "(none)"
		}
		return l[0].MailboxName + "@" + l[0].HostName
	}
	switch fld {
	case "to":
		return addr(m.Envelope.To)
	case "subject":
		s := m.Envelope.Subject
		if len(s) > 60 {
			s = s[:57] + "…"
		}
		return s
	default:
		return addr(m.Envelope.From)
	}
}

func trim(s string) string {
	if len(s) <= 40 {
		return s
	}
	return s[:37] + "…"
}

func wipe(cli *client.Client, sets map[string][]uint32) {
	for folder, ids := range sets {
		_, _ = cli.Select(folder, false)
		ss := new(imap.SeqSet)
		ss.AddNum(ids...)
		cli.Store(ss, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil)
		cli.Expunge(nil)
	}
	fmt.Println("✓ deleted")
}

/* ───── main ───── */

func main() {
	flag.Parse()
	if *email == "" || *password == "" {
		flag.Usage()
		os.Exit(1)
	}

	statsMode := *match == ""
	sizeOn := !statsMode || *sizeFlag // size always on when match mode

	// find server & connect
	server := *imapHost
	if server == "" {
		var err error
		server, err = guessServer(*email)
		if err != nil {
			log.Fatal(err)
		}
	}
	cli, err := dialSmart(server)
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Logout()
	if err := cli.Login(*email, *password); err != nil {
		log.Fatal("login:", err)
	}

	if sizeOn {
		fmt.Println("📏 Size counting: ON")
	} else {
		fmt.Println("📏 Size counting: OFF (add -size for MB column)")
	}
	fmt.Println("🔐  Auth OK")

	/* ── discover selectable folders ── */
	folders := []*imap.MailboxInfo{}
	box := make(chan *imap.MailboxInfo, 64)
	go func() { _ = cli.List("", "*", box) }()
	for mb := range box {
		skip := false
		for _, attr := range mb.Attributes {
			if attr == imap.NoSelectAttr {
				skip = true
				break
			}
		}
		if !skip {
			folders = append(folders, mb)
		}
	}
	// always include INBOX
	inboxPresent := false
	for _, f := range folders {
		if strings.EqualFold(f.Name, "INBOX") {
			inboxPresent = true
			break
		}
	}
	if !inboxPresent {
		folders = append(folders, &imap.MailboxInfo{Name: "INBOX"})
	}
	if len(folders) == 0 {
		log.Fatal("No selectable mailboxes")
	}

	/* ── counters ── */
	var processedFolders int32
	var matches int64
	var scanned int64
	var groups int64

	/* ── buckets ── */
	buckets := map[string]*bucket{}
	target := &bucket{Key: *match, ByFolder: map[string][]uint32{}}

	/* ── sequential scan (one folder ⇒ one IMAP command) ── */
	for i, mb := range folders {
		if _, err := cli.Select(mb.Name, false); err != nil {
			atomic.AddInt32(&processedFolders, 1)
			continue
		}
		criteria := imap.NewSearchCriteria()
		if !statsMode {
			criteria.Header.Add(strings.Title(*field), *match)
		}
		ids, _ := cli.Search(criteria)
		if len(ids) == 0 && statsMode {
			criteria = imap.NewSearchCriteria()
			ids, _ = cli.Search(criteria)
		}
		if len(ids) == 0 {
			atomic.AddInt32(&processedFolders, 1)
			continue
		}

		seq := new(imap.SeqSet)
		seq.AddNum(ids...)
		items := []imap.FetchItem{imap.FetchEnvelope}
		if sizeOn {
			items = append(items, imap.FetchRFC822Size)
		}
		msgs := make(chan *imap.Message, 32)
		go func() { _ = cli.Fetch(seq, items, msgs) }()

		for msg := range msgs {
			if msg == nil || msg.Envelope == nil {
				continue
			}
			if statsMode {
				key := classify(msg, *field)
				if buckets[key] == nil {
					buckets[key] = &bucket{Key: key, ByFolder: map[string][]uint32{}}
					atomic.AddInt64(&groups, 1)
				}
				buckets[key].add(mb.Name, msg.SeqNum, int64(msg.Size))
				atomic.AddInt64(&scanned, 1)
			} else { // match mode
				val := strings.ToLower(classify(msg, *field))
				if !strings.Contains(val, strings.ToLower(*match)) {
					continue
				}
				target.add(mb.Name, msg.SeqNum, int64(msg.Size))
				atomic.AddInt64(&matches, 1)
			}
		}
		atomic.AddInt32(&processedFolders, 1)

		/* ── live progress line ── */
		if statsMode {
			fmt.Printf("\r⏳ %2d/%2d folders — msgs: %d — groups: %d",
				processedFolders, len(folders),
				scanned, groups)
		} else {
			fmt.Printf("\r⏳ %2d/%2d folders — matches: %d",
				processedFolders, len(folders), matches)
		}
		_ = i // silence unused
	}
	fmt.Print("\r                                              \r") // clear line

	/* ───── match mode output & delete ───── */
	if !statsMode {
		if target.Count == 0 {
			fmt.Printf("No matches for \"%s\" in %s\n", *match, strings.ToUpper(*field))
			return
		}
		fmt.Printf("\nMatches for \"%s\" in %s\n", *match, strings.ToUpper(*field))
		for f, ids := range target.ByFolder {
			fmt.Printf("  %-36s %6d\n", f, len(ids))
		}
		fmt.Printf("Total: %d messages  (%.1f MB)\n",
			target.Count, float64(target.Bytes)/(1024*1024))
		fmt.Print("Delete them? (y/N): ")
		var in string
		fmt.Scanln(&in)
		if strings.ToLower(in) != "y" {
			return
		}
		wipe(cli, target.ByFolder)
		return
	}

	/* ───── stats mode table & interactive delete ───── */
	type pair struct{ b *bucket }
	var list []pair
	for _, b := range buckets {
		list = append(list, pair{b})
	}
	if len(list) == 0 {
		fmt.Println("No messages found.")
		return
	}
	sort.Slice(list, func(i, j int) bool { return list[i].b.Count > list[j].b.Count })

	page := 0
	for {
		start, end := page*pageSize, (page+1)*pageSize
		if start >= len(list) {
			fmt.Println("End")
			return
		}
		if end > len(list) {
			end = len(list)
		}

		fmt.Printf("\n%s %d‑%d / %d  (group by %s)\n",
			strings.ToUpper(*field), start+1, end, len(list), *field)

		if sizeOn {
			fmt.Println("┌────┬──────────────────────────────────────────┬────────┬────────┐")
			fmt.Printf("│ %-2s │ %-40s │ %-6s │ %-6s │\n", "#", strings.ToUpper(*field), "MSGS", "MB")
			fmt.Println("├────┼──────────────────────────────────────────┼────────┼────────┤")
		} else {
			fmt.Println("┌────┬──────────────────────────────────────────┬────────┐")
			fmt.Printf("│ %-2s │ %-40s │ %-6s │\n", "#", strings.ToUpper(*field), "MSGS")
			fmt.Println("├────┼──────────────────────────────────────────┼────────┤")
		}

		for i := start; i < end; i++ {
			b := list[i].b
			if sizeOn {
				fmt.Printf("│ %2d │ %-40s │ %6d │ %6.1f │\n",
					i-start+1, trim(b.Key), b.Count, float64(b.Bytes)/(1024*1024))
			} else {
				fmt.Printf("│ %2d │ %-40s │ %6d │\n",
					i-start+1, trim(b.Key), b.Count)
			}
		}
		if sizeOn {
			fmt.Println("└────┴──────────────────────────────────────────┴────────┴────────┘")
		} else {
			fmt.Println("└────┴──────────────────────────────────────────┴────────┘")
		}

		fmt.Print("number=delete  n/p=next/prev  q=quit : ")
		var in string
		fmt.Scanln(&in)
		switch strings.ToLower(in) {
		case "n":
			page++
		case "p":
			if page > 0 {
				page--
			}
		case "q":
			return
		default:
			n, err := strconv.Atoi(in)
			if err != nil || n < 1 || n > end-start {
				fmt.Println("bad input")
				continue
			}
			b := list[start+n-1].b
			fmt.Printf("Delete ALL for \"%s\" (%d msgs)? (y/N): ", b.Key, b.Count)
			var conf string
			fmt.Scanln(&conf)
			if strings.ToLower(conf) != "y" {
				continue
			}
			wipe(cli, b.ByFolder)
			list = append(list[:start+n-1], list[start+n:]...)
			if start >= len(list) && page > 0 {
				page--
			}
		}
	}
}

