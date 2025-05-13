// imap_cleaner.go  —  2025‑05‑13
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
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

/* ---------------- Flags ---------------- */

var (
	email       = flag.String("email", "", "Email address")
	password    = flag.String("password", "", "Password")
	imapServer  = flag.String("imap", "", "IMAP server host:port")
	fromFilter  = flag.String("from", "", "Sender address to wipe (omit for -stats)")
	statsMode   = flag.Bool("stats", false, "Statistics mode: list top senders")
	autodelete  = flag.Bool("autodelete", false, "Delete without confirmation")
	allowPlain  = flag.Bool("allow-plain", false, "Allow unencrypted fallback (NOT secure)")
	pageSize    = 20
)

/* ---------- TLS helpers ---------- */

func dialSmart(server string) (*client.Client, error) {
	host, port, _ := net.SplitHostPort(server)
	modern := &tls.Config{ServerName: host, InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}
	legacy := &tls.Config{
		ServerName: host, InsecureSkipVerify: true, MinVersion: tls.VersionTLS10,
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_RC4_128_SHA, tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA, tls.TLS_RSA_WITH_AES_256_CBC_SHA}}
	connect := func(tc *tls.Config) (*client.Client, error) {
		switch port {
		case "993":
			return client.DialTLS(server, tc)
		case "143":
			c, err := client.Dial(server)
			if err != nil {
				return nil, err
			}
			if err := c.StartTLS(tc); err != nil {
				c.Logout()
				return nil, err
			}
			return c, nil
		default:
			return nil, fmt.Errorf("unsupported port %s", port)
		}
	}
	if c, err := connect(modern); err == nil {
		fmt.Println("✅  Modern TLS")
		return c, nil
	}
	if c, err := connect(legacy); err == nil {
		fmt.Println("⚠️  Legacy TLS (weak ciphers)")
		return c, nil
	}
	if port == "143" && *allowPlain {
		fmt.Println("⚠️  Plain IMAP (no TLS)")
		return client.Dial(server)
	}
	return nil, fmt.Errorf("TLS negotiation failed")
}

/* ---------- IMAP helpers ---------- */

func tryIMAPS(addr string) bool {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second},
		"tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return false
	}
	conn.Close(); return true
}

var prefixes = []string{"imap.", "mail.", "mx.", "webmail.", ""}

func guessServer(addr string) (string, error) {
	a, err := mail.ParseAddress(addr)
	if err != nil {
		return "", err
	}
	domain := strings.Split(a.Address, "@")[1]
	for _, p := range prefixes {
		host := p + domain + ":993"
		if tryIMAPS(host) {
			return host, nil
		}
	}
	mx, _ := net.LookupMX(domain)
	if len(mx) > 0 {
		return strings.TrimSuffix(mx[0].Host, ".") + ":993", nil
	}
	return "", fmt.Errorf("can't guess server")
}

/* ---------- Data structures ---------- */

type senderStat struct {
	Name        string
	Count       int
	Bytes       int64
	FolderIDs   map[string][]uint32 // folder -> message IDs
}

/* ---------- Delete helper ---------- */

func deleteSets(cli *client.Client, sets map[string][]uint32) {
	for folder, ids := range sets {
		if len(ids) == 0 {
			continue
		}
		fmt.Printf("  Deleting %d in %s\n", len(ids), folder)
		_, _ = cli.Select(folder, false)
		seq := new(imap.SeqSet); seq.AddNum(ids...)
		_ = cli.Store(seq, imap.FormatFlagsOp(imap.AddFlags, true),
			[]interface{}{imap.DeletedFlag}, nil)
		_ = cli.Expunge(nil)
	}
	fmt.Println("✓ Done")
}

/* ---------- Main ---------- */

func main() {
	flag.Parse()
	if *email == "" || *password == "" {
		flag.Usage(); os.Exit(1)
	}
	if *fromFilter == "" && !*statsMode {
		log.Fatal("Use -from or -stats")
	}
	if *fromFilter != "" && *statsMode {
		log.Fatal("Choose either -from or -stats, not both")
	}

	server := *imapServer
	if server == "" {
		var err error
		server, err = guessServer(*email)
		if err != nil {
			log.Fatal(err)
		}
	}

	cli, err := dialSmart(server)
	if err != nil { log.Fatal(err) }
	defer cli.Logout()

	if err := cli.Login(*email, *password); err != nil {
		log.Fatal("login:", err)
	}
	fmt.Println("🔐  Auth OK")

	/* --------- Scan folders --------- */
	boxes := make(chan *imap.MailboxInfo, 64)
	go func() { _ = cli.List("", "*", boxes) }()

	folderSets := map[string][]uint32{}
	stats := map[string]*senderStat{}

	for box := range boxes {
		if _, err := cli.Select(box.Name, false); err != nil { continue }
		criteria := imap.NewSearchCriteria()
		if *fromFilter != "" { criteria.Header.Add("From", *fromFilter) }
		ids, _ := cli.Search(criteria)
		if len(ids) == 0 && *statsMode {
			criteria = imap.NewSearchCriteria() // all
			ids, _ = cli.Search(criteria)
		}
		if len(ids) == 0 { continue }
		folderSets[box.Name] = ids

		if *statsMode {
			seq := new(imap.SeqSet); seq.AddNum(ids...)
			items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchRFC822Size}
			msgs := make(chan *imap.Message, 64)
			go func() { _ = cli.Fetch(seq, items, msgs) }()
			for msg := range msgs {
				if msg.Envelope == nil || len(msg.Envelope.From) == 0 { continue }
				s := msg.Envelope.From[0]
				sender := s.MailboxName + "@" + s.HostName
				if stats[sender] == nil {
					stats[sender] = &senderStat{Name: sender, FolderIDs: map[string][]uint32{}}
				}
				stat := stats[sender]
				stat.Count++
				stat.Bytes += int64(msg.Size)
				stat.FolderIDs[box.Name] = append(stat.FolderIDs[box.Name], msg.SeqNum)
			}
		}
	}

	/* --------- Single‑sender mode --------- */
	if *fromFilter != "" {
		if len(folderSets) == 0 {
			fmt.Println("No messages from", *fromFilter); return
		}
		fmt.Println("\nMessages from", *fromFilter)
		fmt.Println("┌──────────────────────────────────────────┬────────┐")
		for f, ids := range folderSets { fmt.Printf("│ %-40s │ %6d │\n", f, len(ids)) }
		fmt.Println("└──────────────────────────────────────────┴────────┘")
		if !*autodelete {
			fmt.Print("Delete them? (y/N): ")
			var in string; fmt.Scanln(&in)
			if strings.ToLower(in) != "y" { return }
		}
		deleteSets(cli, folderSets); return
	}

	/* --------- Statistics mode with paging --------- */
	type pair struct{ s *senderStat }
	var list []pair
	for _, st := range stats { list = append(list, pair{st}) }
	sort.Slice(list, func(i, j int) bool { return list[i].s.Count > list[j].s.Count })

	page := 0
	for {
		start := page * pageSize
		if start >= len(list) { fmt.Println("End of list"); break }
		end := start + pageSize
		if end > len(list) { end = len(list) }

		/* Print page */
		fmt.Printf("\nSenders %d‑%d of %d:\n", start+1, end, len(list))
		fmt.Println("┌────┬──────────────────────────────────────────┬────────┬────────┐")
		fmt.Println("│ #  │ SENDER                                   │  MSGS  │  MB   │")
		fmt.Println("├────┼──────────────────────────────────────────┼────────┼────────┤")
		for i := start; i < end; i++ {
			st := list[i].s
			mb := float64(st.Bytes) / (1024 * 1024)
			fmt.Printf("│ %2d │ %-40s │ %6d │ %6.1f │\n", i-start+1, st.Name, st.Count, mb)
		}
		fmt.Println("└────┴──────────────────────────────────────────┴────────┴────────┘")
		fmt.Print("Enter # to delete, n=next, p=prev, q=quit: ")
		var in string
		fmt.Scanln(&in)
		switch strings.ToLower(in) {
		case "n": page++
		case "p": if page > 0 { page-- }
		case "q": return
		default:
			num, err := strconv.Atoi(in)
			if err != nil || num < 1 || num > end-start {
				fmt.Println("Invalid input"); continue
			}
			st := list[start+num-1].s
			if !*autodelete {
				fmt.Printf("Delete ALL from %s (%d msg / %.1f MB)? (y/N): ",
					st.Name, st.Count, float64(st.Bytes)/(1024*1024))
				var conf string; fmt.Scanln(&conf)
				if strings.ToLower(conf) != "y" { continue }
			}
			deleteSets(cli, st.FolderIDs)
			// remove sender from list
			list = append(list[:start+num-1], list[start+num:]...)
		}
	}
}

