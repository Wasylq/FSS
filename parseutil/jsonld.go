package parseutil

import (
	"encoding/json"
	"regexp"
	"strings"
)

// VideoObject holds the common fields from a schema.org VideoObject
// embedded in a page's JSON-LD script block.
type VideoObject struct {
	URL           string   `json:"url"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	ThumbnailURL  string   `json:"thumbnailUrl"`
	ContentURL    string   `json:"contentUrl"`
	Duration      string   `json:"duration"`
	UploadDate    string   `json:"uploadDate"`
	DatePublished string   `json:"datePublished"`
	Actors        []string `json:"-"`
	Director      string   `json:"-"`
	Keywords      string   `json:"keywords"`
	PartOfSeries  string   `json:"-"`
}

type rawVideoObject struct {
	Type          string          `json:"@type"`
	URL           string          `json:"url"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	ThumbnailURL  string          `json:"thumbnailUrl"`
	ContentURL    string          `json:"contentUrl"`
	Duration      string          `json:"duration"`
	UploadDate    string          `json:"uploadDate"`
	DatePublished string          `json:"datePublished"`
	Actor         json.RawMessage `json:"actor"`
	Director      json.RawMessage `json:"director"`
	Keywords      string          `json:"keywords"`
	PartOfSeries  *struct {
		Name string `json:"name"`
	} `json:"partOfSeries"`
}

type rawItemList struct {
	Type     string `json:"@type"`
	Elements []struct {
		Item rawVideoObject `json:"item"`
	} `json:"itemListElement"`
}

var jsonLDBlockRe = regexp.MustCompile(`(?s)<script[^>]+type="application/ld\+json"[^>]*>(.*?)</script>`)

// ExtractVideoObject finds the first VideoObject in the page's JSON-LD
// blocks. Returns nil if none is found. It handles both bare VideoObject
// blocks and ItemList wrappers (returning the first item). Actor fields
// are parsed flexibly: arrays of strings, arrays of {"name":"…"} objects,
// or a single string all work.
func ExtractVideoObject(body []byte) *VideoObject {
	vos := extractVideoObjects(body, true)
	if len(vos) == 0 {
		return nil
	}
	return &vos[0]
}

// ExtractVideoObjects returns all VideoObject entries found in the
// page's JSON-LD blocks, including those wrapped in an ItemList.
func ExtractVideoObjects(body []byte) []VideoObject {
	return extractVideoObjects(body, false)
}

func extractVideoObjects(body []byte, firstOnly bool) []VideoObject {
	var result []VideoObject
	for _, m := range jsonLDBlockRe.FindAllSubmatch(body, -1) {
		raw := m[1]

		var probe struct {
			Type string `json:"@type"`
		}
		if json.Unmarshal(raw, &probe) != nil {
			continue
		}

		switch probe.Type {
		case "VideoObject":
			var rvo rawVideoObject
			if json.Unmarshal(raw, &rvo) != nil {
				continue
			}
			result = append(result, convertRaw(rvo))
			if firstOnly {
				return result
			}

		case "ItemList":
			var il rawItemList
			if json.Unmarshal(raw, &il) != nil {
				continue
			}
			for _, elem := range il.Elements {
				if elem.Item.Type == "VideoObject" {
					result = append(result, convertRaw(elem.Item))
					if firstOnly {
						return result
					}
				}
			}
		}
	}
	return result
}

func convertRaw(rvo rawVideoObject) VideoObject {
	vo := VideoObject{
		URL:           rvo.URL,
		Name:          rvo.Name,
		Description:   rvo.Description,
		ThumbnailURL:  rvo.ThumbnailURL,
		ContentURL:    rvo.ContentURL,
		Duration:      rvo.Duration,
		UploadDate:    rvo.UploadDate,
		DatePublished: rvo.DatePublished,
		Keywords:      rvo.Keywords,
	}
	if rvo.PartOfSeries != nil {
		vo.PartOfSeries = rvo.PartOfSeries.Name
	}
	vo.Actors = parseFlexActors(rvo.Actor)
	vo.Director = parseFlexPerson(rvo.Director)
	return vo
}

// parseFlexActors handles three JSON shapes for the actor field:
//   - array of strings: ["Alice","Bob"]
//   - array of objects: [{"name":"Alice"},{"name":"Bob"}]
//   - single string:    "Alice"
//   - single object:    {"name":"Alice"}
func parseFlexActors(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil
	}

	if raw[0] == '[' {
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) != nil {
			return nil
		}
		var names []string
		for _, elem := range arr {
			elem = []byte(strings.TrimSpace(string(elem)))
			if len(elem) == 0 {
				continue
			}
			switch elem[0] {
			case '"':
				var s string
				if json.Unmarshal(elem, &s) == nil {
					if n := strings.TrimSpace(s); n != "" {
						names = append(names, n)
					}
				}
			case '{':
				var obj struct {
					Name string `json:"name"`
				}
				if json.Unmarshal(elem, &obj) == nil {
					if n := strings.TrimSpace(obj.Name); n != "" {
						names = append(names, n)
					}
				}
			}
		}
		return names
	}

	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			if n := strings.TrimSpace(s); n != "" {
				return []string{n}
			}
		}
	}

	if raw[0] == '{' {
		var obj struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(raw, &obj) == nil {
			if n := strings.TrimSpace(obj.Name); n != "" {
				return []string{n}
			}
		}
	}

	return nil
}

func parseFlexPerson(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var obj struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return strings.TrimSpace(obj.Name)
	}
	return ""
}
