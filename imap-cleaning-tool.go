// imap-cleaning-tool.go  ·  2025‑05‑13
//
// IMAP cleaner / statistics / backup‑restore with live progress lines.
//
//  Flags
//    -email  user@example.com   ·required
//    -password  ***             ·required
//    -imap host:port            (auto‑guess if omitted)
//    -field from|to|subject     (stats & -match)   default: from
//    -match "text"              (delete interactively)
//    -size                      (add MB column to stats)
//    -backup   mailbox.tgz      (make backup & exit)
//    -restore  mailbox.tgz      (restore & exit)
//    -allow-plain               (allow PLAINTEXT on :143)
//
//  Typical runs
//    go run imap-cleaning-tool.go -email you -password pw -match spam
//    go run imap-cleaning-tool.go -email you -password pw -size
//    go run imap-cleaning-tool.go -email you -password pw -backup all.tgz
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

/* ── flags ─────────────────────────────────────────────── */

var (
	emailF    = flag.String("email", "", "Email")
	passF     = flag.String("password", "", "Password")
	imapF     = flag.String("imap", "", "IMAP host:port")
	fieldF    = flag.String("field", "from", "from | to | subject")
	matchF    = flag.String("match", "", "Text to match in FIELD")
	sizeF     = flag.Bool("size", false, "Add MB column in stats")
	backupF   = flag.String("backup", "", "Create backup & exit")
	restoreF  = flag.String("restore", "", "Restore backup & exit")
	allowPlnF = flag.Bool("allow-plain", false, "Allow PLAINTEXT on 143")
	pageSz    = 20
)

/* ── helper funcs ───────────────────────────────────────── */

func trim(s string) string {
	if len(s) <= 40 {
		return s
	}
	return s[:37] + "…"
}
func classify(m *imap.Message, fld string) string {
	addr := func(a []*imap.Address) string {
		if len(a) == 0 {
			return "(none)"
		}
		return a[0].MailboxName + "@" + a[0].HostName
	}
	switch fld {
	case "to":
		return addr(m.Envelope.To)
	case "subject":
		sub := m.Envelope.Subject
		if len(sub) > 60 {
			sub = sub[:57] + "…"
		}
		return sub
	default:
		return addr(m.Envelope.From)
	}
}

/* ── stats bucket ──────────────────────────────────────── */

type bucket struct {
	Key      string
	Cnt      int
	Bytes    int64
	ByFolder map[string][]uint32
}

func (b *bucket) add(folder string, uid uint32, sz int64) {
	b.Cnt++
	b.Bytes += sz
	b.ByFolder[folder] = append(b.ByFolder[folder], uid)
}

/* ── safe delete ───────────────────────────────────────── */

func wipe(cli *client.Client, sets map[string][]uint32) {
	for f, ids := range sets {
		cli.Select(f, false)
		ss := new(imap.SeqSet)
		ss.AddNum(ids...)
		cli.Store(ss, imap.FormatFlagsOp(imap.AddFlags, true),
			[]interface{}{imap.DeletedFlag}, nil)
		cli.Expunge(nil)
	}
	fmt.Println("✓ deleted")
}

/* ── TLS / connect helpers ─────────────────────────────── */

func dialSmart(addr string) (*client.Client, error) {
	host, port, _ := net.SplitHostPort(addr)
	mod := &tls.Config{ServerName: host, InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}
	leg := &tls.Config{ServerName: host, InsecureSkipVerify: true, MinVersion: tls.VersionTLS10,
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_RC4_128_SHA,
			tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA}}
	connect := func(c *tls.Config) (*client.Client, error) {
		switch port {
		case "993":
			return client.DialTLS(addr, c)
		case "143":
			cl, err := client.Dial(addr)
			if err != nil {
				return nil, err
			}
			if err = cl.StartTLS(c); err != nil {
				cl.Logout()
				return nil, err
			}
			return cl, nil
		}
		return nil, fmt.Errorf("unsupported port")
	}
	if c, err := connect(mod); err == nil {
		fmt.Println("✅  Modern TLS")
		return c, nil
	}
	if c, err := connect(leg); err == nil {
		fmt.Println("⚠️  Legacy TLS")
		return c, nil
	}
	if port == "143" && *allowPlnF {
		fmt.Println("⚠️  Plain IMAP")
		return client.Dial(addr)
	}
	return nil, fmt.Errorf("TLS failed")
}

func guessServer(email string) string {
	d := strings.Split(email, "@")[1]
	for _, p := range []string{"imap.", "mail.", ""} {
		h := p + d + ":993"
		if _, err := tls.Dial("tcp", h, &tls.Config{InsecureSkipVerify: true}); err == nil {
			return h
		}
	}
	return d + ":143"
}

/* ── backup & restore ─────────────────────────────────── */

func backupAll(cli *client.Client, tgz string) error {
	f, err := os.Create(tgz)
	if err != nil {
		return err
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	var folders, msgs int64
	mbc := make(chan *imap.MailboxInfo, 64)
	go func() { _ = cli.List("", "*", mbc) }()
	for mb := range mbc {
		sel := true
		for _, a := range mb.Attributes {
			if a == imap.NoSelectAttr {
				sel = false
			}
		}
		if !sel {
			continue
		}
		if _, e := cli.Select(mb.Name, false); e != nil {
			continue
		}
		uids, _ := cli.Search(imap.NewSearchCriteria())
		if len(uids) == 0 {
			continue
		}
		folders++
		seq := new(imap.SeqSet)
		seq.AddNum(uids...)
		msgCh := make(chan *imap.Message, 32)
		go func() { _ = cli.Fetch(seq, []imap.FetchItem{imap.FetchUid, imap.FetchRFC822}, msgCh) }()
		for m := range msgCh {
			if m == nil {
				continue
			}
			data, _ := io.ReadAll(m.GetBody(&imap.BodySectionName{}))
			h := &tar.Header{Name: fmt.Sprintf("%s/%d.eml", mb.Name, m.Uid), Size: int64(len(data)), Mode: 0600}
			tw.WriteHeader(h)
			tw.Write(data)
			msgs++
			fmt.Printf("\r📦 Backup folders:%d msgs:%d", folders, msgs)
		}
	}
	fmt.Print("\r                                        \r")
	return nil
}

func restoreAll(cli *client.Client, tgz string) error {
	f, err := os.Open(tgz)
	if err != nil {
		return err
	}
	defer f.Close()
	gr, _ := gzip.NewReader(f)
	defer gr.Close()
	tr := tar.NewReader(gr)

	var restored int64
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if h.FileInfo().IsDir() {
			continue
		}
		fold := filepath.Dir(h.Name)
		if fold == "." {
			fold = "INBOX"
		}
		cli.Create(fold)
		data, _ := io.ReadAll(tr)
		cli.Append(fold, nil, time.Now(), bytes.NewReader(data))
		restored++
		fmt.Printf("\r⬆️ Restore msgs:%d", restored)
	}
	fmt.Print("\r                                   \r")
	return nil
}

/* ── main ─────────────────────────────────────────────── */

func main() {
	flag.Parse()
	if *emailF == "" || *passF == "" {
		flag.Usage()
		return
	}
	if (*backupF != "" || *restoreF != "") && *matchF != "" {
		log.Fatal("-match cannot be combined with backup/restore")
	}

	// connect
	host := *imapF
	if host == "" {
		host = guessServer(*emailF)
	}
	cli, err := dialSmart(host)
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Logout()
	if err := cli.Login(*emailF, *passF); err != nil {
		log.Fatal("login:", err)
	}

	/* backup / restore shortcuts */
	if *backupF != "" {
		fmt.Println("🔄 Backup →", *backupF)
		if err := backupAll(cli, *backupF); err != nil {
			log.Fatal(err)
		}
		fmt.Println("✓ backup done")
		return
	}
	if *restoreF != "" {
		fmt.Println("🔄 Restore ←", *restoreF)
		if err := restoreAll(cli, *restoreF); err != nil {
			log.Fatal(err)
		}
		fmt.Println("✓ restore done")
		return
	}

	statsMode := *matchF == ""
	sizeOn := !statsMode || *sizeF
	if sizeOn {
		fmt.Println("📏 Size counting ON")
	}

	/* discover selectable folders */
	folders := []string{"INBOX"}
	mbCh := make(chan *imap.MailboxInfo, 64)
	go func() { _ = cli.List("", "*", mbCh) }()
	for mb := range mbCh {
		selectable := true
		for _, a := range mb.Attributes {
			if a == imap.NoSelectAttr {
				selectable = false
				break
			}
		}
		if selectable && mb.Name != "INBOX" {
			folders = append(folders, mb.Name)
		}
	}

	buckets := map[string]*bucket{}
	target := &bucket{Key: *matchF, ByFolder: map[string][]uint32{}}
	var totMsgs, matchMsgs int64

	for i, folder := range folders {
		cli.Select(folder, false)
		crit := imap.NewSearchCriteria()
		if !statsMode {
			crit.Header.Add(strings.Title(*fieldF), *matchF)
		}
		uids, _ := cli.Search(crit)
		if len(uids) == 0 && statsMode {
			crit = imap.NewSearchCriteria()
			uids, _ = cli.Search(crit)
		}
		if len(uids) == 0 {
			continue
		}
		seq := new(imap.SeqSet)
		seq.AddNum(uids...)
		items := []imap.FetchItem{imap.FetchEnvelope}
		if sizeOn {
			items = append(items, imap.FetchRFC822Size)
		}
		mc := make(chan *imap.Message, 32)
		go func() { _ = cli.Fetch(seq, items, mc) }()
		for m := range mc {
			if statsMode {
				key := classify(m, *fieldF)
				if buckets[key] == nil {
					buckets[key] = &bucket{Key: key, ByFolder: map[string][]uint32{}}
				}
				buckets[key].add(folder, m.SeqNum, int64(m.Size))
				totMsgs++
			} else if strings.Contains(strings.ToLower(classify(m, *fieldF)), strings.ToLower(*matchF)) {
				target.add(folder, m.SeqNum, int64(m.Size))
				matchMsgs++
			}
		}
		if statsMode {
			fmt.Printf("\r⏳ %2d/%2d folders  msgs:%d", i+1, len(folders), totMsgs)
		} else {
			fmt.Printf("\r⏳ %2d/%2d folders  matches:%d", i+1, len(folders), matchMsgs)
		}
	}
	fmt.Print("\r                                             \r")

	/* match mode output & delete */
	if !statsMode {
		if target.Cnt == 0 {
			fmt.Println("Nothing matches")
			return
		}
		fmt.Printf("\nMatches for \"%s\" (%s)\n", *matchF, *fieldF)
		for f, ids := range target.ByFolder {
			fmt.Printf("  %-35s %6d\n", f, len(ids))
		}
		fmt.Printf("Total: %d msgs  %.1f MB\n", target.Cnt, float64(target.Bytes)/(1024*1024))
		fmt.Print("Delete? (y/N): ")
		var ans string
		fmt.Scanln(&ans)
		if strings.ToLower(ans) == "y" {
			wipe(cli, target.ByFolder)
		}
		return
	}

	/* stats mode table */
	type pair struct{ b *bucket }
	var list []pair
	for _, v := range buckets {
		list = append(list, pair{v})
	}
	if len(list) == 0 {
		fmt.Println("Mailbox empty")
		return
	}
	sort.Slice(list, func(i, j int) bool { return list[i].b.Cnt > list[j].b.Cnt })

	page := 0
	for {
		start, end := page*pageSz, (page+1)*pageSz
		if start >= len(list) {
			fmt.Println("End")
			return
		}
		if end > len(list) {
			end = len(list)
		}
		fmt.Printf("\n%s %d‑%d / %d\n", strings.ToUpper(*fieldF), start+1, end, len(list))
		if sizeOn {
			fmt.Println("┌────┬──────────────────────────────────────────┬────────┬────────┐")
			fmt.Printf("│ # │ %-40s │ MSGS │  MB │\n", strings.ToUpper(*fieldF))
			fmt.Println("├────┼──────────────────────────────────────────┼────────┼────────┤")
		} else {
			fmt.Println("┌────┬──────────────────────────────────────────┬────────┐")
			fmt.Printf("│ # │ %-40s │ MSGS │\n", strings.ToUpper(*fieldF))
			fmt.Println("├────┼──────────────────────────────────────────┼────────┤")
		}
		for i := start; i < end; i++ {
			b := list[i].b
			if sizeOn {
				fmt.Printf("│ %2d │ %-40s │ %6d │ %6.1f │\n", i-start+1, trim(b.Key), b.Cnt, float64(b.Bytes)/(1024*1024))
			} else {
				fmt.Printf("│ %2d │ %-40s │ %6d │\n", i-start+1, trim(b.Key), b.Cnt)
			}
		}
		if sizeOn {
			fmt.Println("└────┴──────────────────────────────────────────┴────────┴────────┘")
		} else {
			fmt.Println("└────┴──────────────────────────────────────────┴────────┘")
		}
		fmt.Print("num=del  n/p  q : ")
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
			idx, err := strconv.Atoi(in)
			if err != nil || idx < 1 || idx > end-start {
				fmt.Println("bad input")
				continue
			}
			b := list[start+idx-1].b
			fmt.Printf("Delete ALL for \"%s\" (%d)? (y/N): ", b.Key, b.Cnt)
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) == "y" {
				wipe(cli, b.ByFolder)
				list = append(list[:start+idx-1], list[start+idx:]...)
				if start >= len(list) && page > 0 {
					page--
				}
			}
		}
	}
}

