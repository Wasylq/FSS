package glamosetour

import "github.com/Wasylq/FSS/scraper"

// Glamose network tour sites using the refstat.php template.
// Each shows ~20-24 recent entries without historical pagination.
//
// Dead/gone sites not covered:
//   - laceybanghardonline.com — ECONNREFUSED
//   - lyciasharyl.com — 301 redirect to unrelated pharmacy site
//   - pinupwow.com — 301 redirect to Shopify store (glamour photos, not porn)
var sites = []SiteConfig{
	{"damselsinperil", "damselsinperil.com", "Damsels In Peril"},
	{"demurefun", "demurefun.com", "Demure Fun"},
	{"downblousewow", "downblousewow.com", "Down Blouse Wow"},
	{"femmefight", "femmefight.com", "Femme Fight"},
	{"hotpantyfun", "hotpantyfun.com", "Hot Panty Fun"},
	{"lethallipstick", "lethallipstick.com", "Lethal Lipstick"},
	{"pantyamateur", "pantyamateur.com", "Panty Amateur"},
	{"pantymaniacs", "pantymaniacs.com", "Panty Maniacs"},
	{"ripping4fun", "ripping4fun.com", "Ripping 4 Fun"},
	{"satinsilkfun", "satinsilkfun.com", "Satin Silk Fun"},
	{"skirtsupgirls", "skirtsupgirls.com", "Skirts Up Girls"},
	{"ukupskirts", "ukupskirts.com", "UK Upskirts"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}
