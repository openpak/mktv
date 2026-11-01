// Command mktv serves openkart.openpak.org, OpenPak's OpenKart site for Mario Kart 8 (Wii U).
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/openpak/mktv/internal/site"
)

func main() {
	cfg, err := site.ConfigFromEnv(os.Getenv)
	if err != nil {
		log.Fatalf("mktv: %v", err)
	}
	h, err := site.New(cfg)
	if err != nil {
		log.Fatalf("mktv: %v", err)
	}
	srv := &http.Server{Addr: cfg.Addr, Handler: h, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("mktv: listening on %s, API %s", cfg.Addr, cfg.APIURL)
	log.Fatal(srv.ListenAndServe())
}
