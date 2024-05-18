package cmd

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"

	"github.com/urfave/cli/v2"

	"github.com/naiba/nb/internal"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, cloudflareCmd)
}

var cloudflareCmd = &cli.Command{
	Name:        "cloudflare",
	Description: "Run a web interface for bulk management of DNS records, Page Rules, and Rulesets.",
	Action: func(c *cli.Context) error {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			panic(err)
		}
		mux := http.NewServeMux()
		mux.Handle("/", http.HandlerFunc(internal.Cloudflared.Index))
		mux.Handle("/state", http.HandlerFunc(internal.Cloudflared.State))
		mux.Handle("/load-zone-records", http.HandlerFunc(internal.Cloudflared.LoadZoneRecords))
		mux.Handle("/load-zone-page-rules", http.HandlerFunc(internal.Cloudflared.LoadZonePageRules))
		mux.Handle("/load-zone-rulesets", http.HandlerFunc(internal.Cloudflared.LoadZoneRulesets))
		mux.Handle("/delete-dns-records", http.HandlerFunc(internal.Cloudflared.DeleteDNSRecords))
		mux.Handle("/delete-page-rules", http.HandlerFunc(internal.Cloudflared.DeletePageRules))
		mux.Handle("/delete-rulesets", http.HandlerFunc(internal.Cloudflared.DeleteRulesets))
		mux.Handle("/batch-create-dns-record", http.HandlerFunc(internal.Cloudflared.BatchCreteDNSRecord))
		mux.Handle("/batch-create-page-rule", http.HandlerFunc(internal.Cloudflared.BatchCreatePageRule))
		mux.Handle("/batch-create-ruleset", http.HandlerFunc(internal.Cloudflared.BatchCreateRuleset))
		mux.Handle("/check-token", http.HandlerFunc(internal.Cloudflared.CheckToken))
		var errCh = make(chan error)
		go func() {
			errCh <- http.Serve(listener, mux)
		}()
		httpServer := fmt.Sprintf("http://localhost:%d", listener.Addr().(*net.TCPAddr).Port)
		log.Printf("Listening on %s", httpServer)
		exec.Command("open", httpServer).Run()
		return <-errCh
	},
}
