package digitaljmediautil

import (
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
)

// newScene builds a scene shell with the shared identity fields populated.
func newScene(cfg SiteConfig, studioURL, id string, now time.Time) models.Scene {
	return models.Scene{
		ID:        id,
		SiteID:    cfg.SiteID,
		StudioURL: studioURL,
		Studio:    cfg.Studio,
		URL:       cfg.Base + cfg.ListPath,
		ScrapedAt: now,
	}
}

// thumb builds a CDN thumbnail URL for an id under the given segment/filename.
func thumb(cfg SiteConfig, seg, id, file string) string {
	return cdnBase(cfg.Base) + "/" + seg + "/" + id + "/" + file
}

// ---- Fellatio Japan ----
// Block: <div class="tour-data"> with model h2 + "12:22 / 121 photos / tag..",
// followed by a player carrying preview/{id}/sample.mp4.

var (
	fjGirlRe = regexp.MustCompile(`<h2><a href="girl/[^"]*">([^<]+)</a></h2>`)
	fjDataRe = regexp.MustCompile(`<div class="data silver">(.*?)</div>`)
	fjDurRe  = regexp.MustCompile(`(\d{1,2}:\d{2}(?::\d{2})?)`)
	fjTagRe  = regexp.MustCompile(`<a href="tag/[^"]*">([^<]+)</a>`)
	fjIDRe   = regexp.MustCompile(`preview/([^/"]+)/sample\.mp4`)
)

func parseFellatio(cfg SiteConfig, body, studioURL string, now time.Time) []models.Scene {
	blocks := strings.Split(body, `<div class="tour-data">`)
	var out []models.Scene
	for _, b := range blocks[1:] {
		m := fjIDRe.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		sc := newScene(cfg, studioURL, m[1], now)
		sc.Thumbnail = thumb(cfg, "preview", m[1], "scene-lg.jpg")
		if g := fjGirlRe.FindStringSubmatch(b); g != nil {
			name := strings.TrimSpace(html.UnescapeString(g[1]))
			sc.Performers = []string{name}
			sc.Title = name
		}
		if d := fjDataRe.FindStringSubmatch(b); d != nil {
			data := d[1]
			if dm := fjDurRe.FindStringSubmatch(data); dm != nil {
				sc.Duration = parseutil.ParseDurationColon(dm[1])
			}
			var tags []string
			for _, t := range fjTagRe.FindAllStringSubmatch(data, -1) {
				tags = append(tags, t[1])
			}
			sc.Tags = dedupTrim(tags)
		}
		out = append(out, sc)
	}
	return out
}

// ---- Cospuri / Cute Butts share the "scene-thumb" grid CMS ----

var (
	cpIDRe      = regexp.MustCompile(`preview/([^/"]+)/scene-med`)
	cpModelRe   = regexp.MustCompile(`<a href="/model/[^"]*">([^<]+)</a>`)
	cpChannelRe = regexp.MustCompile(`<a class="channel" href="[^"]*">([^<]+)</a>`)
	cpLenRe     = regexp.MustCompile(`<div class="length"><strong>(\d+)</strong>`)
	cpTagRe     = regexp.MustCompile(`<a class="tag" href="[^"]*">([^<]+)</a>`)
)

func parseCospuri(cfg SiteConfig, body, studioURL string, now time.Time) []models.Scene {
	blocks := strings.Split(body, `<div class="scene `)
	var out []models.Scene
	for _, b := range blocks[1:] {
		m := cpIDRe.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		id := m[1]
		sc := newScene(cfg, studioURL, id, now)
		sc.URL = cfg.Base + "/sample?id=" + id
		sc.Thumbnail = thumb(cfg, "preview", id, "scene-lg.jpg")
		var model, channel string
		if mm := cpModelRe.FindStringSubmatch(b); mm != nil {
			model = strings.TrimSpace(html.UnescapeString(mm[1]))
			sc.Performers = []string{model}
		}
		if cm := cpChannelRe.FindStringSubmatch(b); cm != nil {
			channel = strings.TrimSpace(html.UnescapeString(cm[1]))
		}
		sc.Title = strings.TrimSpace(strings.TrimRight(model+" - "+channel, " -"))
		if channel != "" {
			sc.Series = channel
		}
		if lm := cpLenRe.FindStringSubmatch(b); lm != nil {
			sc.Duration = atoi(lm[1]) * 60
		}
		var tags []string
		for _, t := range cpTagRe.FindAllStringSubmatch(b, -1) {
			tags = append(tags, t[1])
		}
		sc.Tags = dedupTrim(tags)
		out = append(out, sc)
	}
	return out
}

// ---- Cute Butts ----
// Block: <div class="scene"> with scene-thumb (preview/{id}), date tag, id link,
// tag-box, <h3 class="title">, <h4 class="model">. Detail page adds Runtime.

var (
	cbIDRe    = regexp.MustCompile(`preview/([^/"]+)/scene-med`)
	cbDateRe  = regexp.MustCompile(`<div class="date tag">([^<]+)</div>`)
	cbURLRe   = regexp.MustCompile(`<a class="id" href="([^"]+)">`)
	cbTitleRe = regexp.MustCompile(`<h3 class="title"><a [^>]*>([^<]+)</a>`)
	cbModelRe = regexp.MustCompile(`<a href="/model/[^"]*">([^<]+)</a>`)
	cbTagRe   = regexp.MustCompile(`<a class="tag" href="[^"]*">([^<]+)</a>`)
	cbRunRe   = regexp.MustCompile(`<strong>Runtime:</strong>\s*(\d+)\s*min`)
)

func parseCuteButts(cfg SiteConfig, body, studioURL string, now time.Time) []models.Scene {
	blocks := strings.Split(body, `<div class="scene">`)
	var out []models.Scene
	for _, b := range blocks[1:] {
		m := cbIDRe.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		id := m[1]
		sc := newScene(cfg, studioURL, id, now)
		sc.Thumbnail = thumb(cfg, "preview", id, "scene-lg.jpg")
		if u := cbURLRe.FindStringSubmatch(b); u != nil {
			sc.URL = cfg.Base + u[1]
		}
		if t := cbTitleRe.FindStringSubmatch(b); t != nil {
			sc.Title = strings.TrimSpace(html.UnescapeString(t[1]))
		}
		if d := cbDateRe.FindStringSubmatch(b); d != nil {
			sc.Date = parseJPDate(d[1])
		}
		var models_ []string
		for _, mm := range cbModelRe.FindAllStringSubmatch(b, -1) {
			models_ = append(models_, mm[1])
		}
		sc.Performers = dedupTrim(models_)
		var tags []string
		for _, t := range cbTagRe.FindAllStringSubmatch(b, -1) {
			tags = append(tags, t[1])
		}
		sc.Tags = dedupTrim(tags)
		out = append(out, sc)
	}
	return out
}

func cuteButtsDetail(scene *models.Scene, body string) {
	if m := cbRunRe.FindStringSubmatch(body); m != nil {
		scene.Duration = atoi(m[1]) * 60
	}
}

// ---- Cum Buffet ----
// Block: <div class="video"> with thumb img preview/{id}, video-link title,
// model-name, date "Jan 12, 2024". Detail page adds tags.

var (
	bufIDRe    = regexp.MustCompile(`preview/([^/"]+)/scene-sm`)
	bufURLRe   = regexp.MustCompile(`<a href="(/sample/[^"]+)"`)
	bufTitleRe = regexp.MustCompile(`class="video-link">([^<]+)</a>`)
	bufModelRe = regexp.MustCompile(`<div class="model-name">(.*?)</div>`)
	bufNameRe  = regexp.MustCompile(`<a href="[^"]*">([^<]+)</a>`)
	bufDateRe  = regexp.MustCompile(`<div class="date">([^<]+)</div>`)
	bufTagRe   = regexp.MustCompile(`<a href="[^"]*" class="tag">([^<]+)</a>`)
)

func parseCumBuffet(cfg SiteConfig, body, studioURL string, now time.Time) []models.Scene {
	blocks := strings.Split(body, `<div class="video">`)
	var out []models.Scene
	for _, b := range blocks[1:] {
		m := bufIDRe.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		id := m[1]
		sc := newScene(cfg, studioURL, id, now)
		sc.Thumbnail = thumb(cfg, "preview", id, "scene-lg.jpg")
		if u := bufURLRe.FindStringSubmatch(b); u != nil {
			sc.URL = cfg.Base + u[1]
		}
		if t := bufTitleRe.FindStringSubmatch(b); t != nil {
			sc.Title = strings.TrimSpace(html.UnescapeString(t[1]))
		}
		if mm := bufModelRe.FindStringSubmatch(b); mm != nil {
			var names []string
			for _, n := range bufNameRe.FindAllStringSubmatch(mm[1], -1) {
				names = append(names, n[1])
			}
			sc.Performers = dedupTrim(names)
		}
		if d := bufDateRe.FindStringSubmatch(b); d != nil {
			if t, err := time.Parse("Jan 2, 2006", strings.TrimSpace(d[1])); err == nil {
				sc.Date = t.UTC()
			}
		}
		out = append(out, sc)
	}
	return out
}

func cumBuffetDetail(scene *models.Scene, body string) {
	var tags []string
	for _, t := range bufTagRe.FindAllStringSubmatch(body, -1) {
		tags = append(tags, t[1])
	}
	if len(tags) > 0 {
		scene.Tags = dedupTrim(tags)
	}
}

// ---- Legs Japan ----
// Block: player (samples/{id}/sample.mp4) + tContent with model h1, title h3,
// length:<strong>, photos:<strong>, tags.

var (
	ljIDRe    = regexp.MustCompile(`samples/([^/"]+)/sample\.mp4`)
	ljModelRe = regexp.MustCompile(`<a href="girl/[^"]*"><h1>([^<]+)</h1>`)
	ljTitleRe = regexp.MustCompile(`<h3><strong>([^<]+)</strong></h3>`)
	ljLenRe   = regexp.MustCompile(`length:<strong>([0-9:]+)</strong>`)
	ljTagRe   = regexp.MustCompile(`<a href="/en/tag/[^"]*">([^<]+)</a>`)
)

func parseLegsJapan(cfg SiteConfig, body, studioURL string, now time.Time) []models.Scene {
	blocks := strings.Split(body, `<div class="player`)
	var out []models.Scene
	for _, b := range blocks[1:] {
		m := ljIDRe.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		id := m[1]
		sc := newScene(cfg, studioURL, id, now)
		sc.Thumbnail = thumb(cfg, "samples", id, "scene-lg.jpg")
		if mm := ljModelRe.FindStringSubmatch(b); mm != nil {
			sc.Performers = []string{strings.TrimSpace(html.UnescapeString(mm[1]))}
		}
		if t := ljTitleRe.FindStringSubmatch(b); t != nil {
			sc.Title = strings.TrimSpace(html.UnescapeString(t[1]))
		}
		if len(sc.Performers) > 0 && sc.Title == "" {
			sc.Title = sc.Performers[0]
		}
		if lm := ljLenRe.FindStringSubmatch(b); lm != nil {
			sc.Duration = parseutil.ParseDurationColon(lm[1])
		}
		var tags []string
		for _, t := range ljTagRe.FindAllStringSubmatch(b, -1) {
			tags = append(tags, t[1])
		}
		sc.Tags = dedupTrim(tags)
		out = append(out, sc)
	}
	return out
}

// ---- Tokyo Face Fuck ----
// Block: <div class="girl box"> with preview/{id}/sample.mp4, info h1 model,
// infotxt description.

var (
	tffIDRe    = regexp.MustCompile(`preview/([^/"]+)/sample\.mp4`)
	tffModelRe = regexp.MustCompile(`<div class="info">\s*<h1>([^<]+)</h1>`)
	tffDescRe  = regexp.MustCompile(`(?s)<div class="infotxt[^"]*">.*?<p>(.*?)</p>`)
)

func parseTokyoFaceFuck(cfg SiteConfig, body, studioURL string, now time.Time) []models.Scene {
	blocks := strings.Split(body, `<div class="girl box">`)
	var out []models.Scene
	for _, b := range blocks[1:] {
		m := tffIDRe.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		id := m[1]
		sc := newScene(cfg, studioURL, id, now)
		sc.Thumbnail = thumb(cfg, "preview", id, "sample.jpg")
		if mm := tffModelRe.FindStringSubmatch(b); mm != nil {
			name := strings.TrimSpace(html.UnescapeString(mm[1]))
			sc.Performers = []string{name}
			sc.Title = name
		}
		if d := tffDescRe.FindStringSubmatch(b); d != nil {
			sc.Description = cleanText(d[1])
		}
		out = append(out, sc)
	}
	return out
}

// ---- Handjob Japan ----
// Block: <div class="item-title"> item-ltitle h1 (comma models), item-rtitle
// Scene Length / Scene Photos, then player preview/{id}/sample.mp4.

var (
	hjIDRe    = regexp.MustCompile(`preview/([^/"]+)/sample\.mp4`)
	hjModelRe = regexp.MustCompile(`<div class="item-ltitle"><h1>([^<]*)<span`)
	hjLenRe   = regexp.MustCompile(`Scene Length <strong>([0-9:]+)</strong>`)
)

func parseHandjobJapan(cfg SiteConfig, body, studioURL string, now time.Time) []models.Scene {
	blocks := strings.Split(body, `<div class="item-title">`)
	var out []models.Scene
	for _, b := range blocks[1:] {
		m := hjIDRe.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		id := m[1]
		sc := newScene(cfg, studioURL, id, now)
		sc.Thumbnail = thumb(cfg, "preview", id, "sample.jpg")
		if mm := hjModelRe.FindStringSubmatch(b); mm != nil {
			sc.Performers = splitModels(mm[1])
			sc.Title = strings.TrimSpace(html.UnescapeString(mm[1]))
		}
		if lm := hjLenRe.FindStringSubmatch(b); lm != nil {
			sc.Duration = parseutil.ParseDurationColon(lm[1])
		}
		out = append(out, sc)
	}
	return out
}

// ---- Sperm Mania ----
// Block: <div class="sample-title"> actress links + free-text title, then player
// preview/{id}/sample.mp4, then sample-info Runtime / Type tag.

var (
	smIDRe    = regexp.MustCompile(`preview/([^/"]+)/sample\.mp4`)
	smTitleRe = regexp.MustCompile(`(?s)<div class="sample-title[^"]*">(.*?)</div>`)
	smActorRe = regexp.MustCompile(`<a href="actress/[^"]*">([^<]+)</a>`)
	smRunRe   = regexp.MustCompile(`Runtime <strong>([0-9:]+)</strong>`)
	smTypeRe  = regexp.MustCompile(`Type <strong><a href="type/[^"]*">([^<]+)</a>`)
)

func parseSpermMania(cfg SiteConfig, body, studioURL string, now time.Time) []models.Scene {
	blocks := strings.Split(body, `<div class="sample-title`)
	var out []models.Scene
	for _, b := range blocks[1:] {
		m := smIDRe.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		id := m[1]
		sc := newScene(cfg, studioURL, id, now)
		sc.Thumbnail = thumb(cfg, "preview", id, "scene-lg.jpg")
		if t := smTitleRe.FindStringSubmatch(`<div class="sample-title` + b); t != nil {
			var actors []string
			for _, a := range smActorRe.FindAllStringSubmatch(t[1], -1) {
				actors = append(actors, a[1])
			}
			sc.Performers = dedupTrim(actors)
			sc.Title = cleanText(t[1])
		}
		if rm := smRunRe.FindStringSubmatch(b); rm != nil {
			sc.Duration = parseutil.ParseDurationColon(rm[1])
		}
		if tm := smTypeRe.FindStringSubmatch(b); tm != nil {
			sc.Tags = dedupTrim([]string{tm[1]})
		}
		out = append(out, sc)
	}
	return out
}

// ---- Transex Japan ----
// Block: <div class="sample-info"> h1 title + "featuring <a..><strong>models</strong></a>
// in <strong>N</strong> photos", followed by player tour/{id}/sample.mp4.

var (
	txIDRe    = regexp.MustCompile(`tour/([0-9a-z]+)/sample\.mp4`)
	txTitleRe = regexp.MustCompile(`<h1>([^<]+)</h1>`)
	txModelRe = regexp.MustCompile(`featuring <a [^>]*><strong>([^<]+)</strong>`)
)

func parseTransexJapan(cfg SiteConfig, body, studioURL string, now time.Time) []models.Scene {
	blocks := strings.Split(body, `<div class="sample-info">`)
	var out []models.Scene
	for _, b := range blocks[1:] {
		m := txIDRe.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		id := m[1]
		sc := newScene(cfg, studioURL, id, now)
		sc.Thumbnail = thumb(cfg, "tour", id, "wide-3.jpg")
		if t := txTitleRe.FindStringSubmatch(b); t != nil {
			sc.Title = strings.TrimSpace(html.UnescapeString(t[1]))
		}
		if mm := txModelRe.FindStringSubmatch(b); mm != nil {
			sc.Performers = splitModels(mm[1])
		}
		out = append(out, sc)
	}
	return out
}

// ---- Ura Lesbian ----
// Block: player tour/{id}/sample.mp4, then <h1> model links, tour-datum Runtime
// ("1H16"), tour-datum Photos.

var (
	ulIDRe    = regexp.MustCompile(`tour/(\d+)/sample\.mp4`)
	ulH1Re    = regexp.MustCompile(`(?s)<h1>(.*?)</h1>`)
	ulModelRe = regexp.MustCompile(`<a href="model/[^"]*">([^<]+)</a>`)
	ulRunRe   = regexp.MustCompile(`Runtime <strong>([0-9HM:]+)</strong>`)
)

func parseUraLesbian(cfg SiteConfig, body, studioURL string, now time.Time) []models.Scene {
	blocks := strings.Split(body, `<div class="player`)
	var out []models.Scene
	for _, b := range blocks[1:] {
		m := ulIDRe.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		id := m[1]
		sc := newScene(cfg, studioURL, id, now)
		sc.Thumbnail = thumb(cfg, "tour", id, "tour-lg.jpg")
		if h := ulH1Re.FindStringSubmatch(b); h != nil {
			var models_ []string
			for _, mm := range ulModelRe.FindAllStringSubmatch(h[1], -1) {
				models_ = append(models_, mm[1])
			}
			sc.Performers = dedupTrim(models_)
			if len(sc.Performers) > 0 {
				sc.Title = strings.Join(sc.Performers, ", ")
			}
		}
		if rm := ulRunRe.FindStringSubmatch(b); rm != nil {
			sc.Duration = parseHMRuntime(rm[1])
		}
		out = append(out, sc)
	}
	return out
}

// parseJPDate parses a "2026・01・23" (or hyphen/slash) date into UTC.
func parseJPDate(s string) time.Time {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("・", "-", ".", "-", "/", "-").Replace(s)
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
