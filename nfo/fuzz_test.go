package nfo

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
)

// FuzzMarshalProducesWellFormedXML feeds scraped-shaped strings through every
// text position in the NFO document and requires the result to parse back as XML.
//
// Every string in a Movie originates from a scraped page — titles, plots,
// performer names, tags — and the thumbnail URL lands in an XML *attribute*
// (Thumb.Aspect) with the URL as chardata, which is a different escaping context
// from element text. A missed escape here does not fail loudly: Kodi and Stash
// just refuse to read the sidecar, so the file looks written and the metadata
// silently never appears.
//
// The check is deliberately "parses back", not "contains no <". encoding/xml is
// expected to escape correctly; the point is to detect any input where it does
// not, or where a future change hand-builds part of the document.
func FuzzMarshalProducesWellFormedXML(f *testing.F) {
	f.Add("Normal Scene Title")
	f.Add("<script>alert(1)</script>")
	f.Add("</movie><movie>")
	f.Add("A & B")
	f.Add(`quote" apostrophe'`)
	f.Add("]]>")
	f.Add("")
	f.Add("emoji 🎬 and CJK 日本語")
	f.Add("\x00\x01\x02")            // control chars are illegal in XML 1.0
	f.Add("￾￿")                      // non-characters
	f.Add(strings.Repeat("x", 1000)) // long
	f.Add("line\nbreak\ttab")

	f.Fuzz(func(t *testing.T, s string) {
		m := Movie{
			Title:      s,
			URLs:       []string{s},
			Premiered:  s,
			Plot:       s,
			Studio:     s,
			Thumbnails: []Thumb{{Aspect: s, URL: s}},
			Actors:     []Actor{{Name: s}},
			Tags:       []string{s},
		}

		out, err := Marshal(m)
		if err != nil {
			// Marshal refusing the input is fine — that is a clean failure the
			// caller can report. Emitting a broken file is not.
			return
		}

		dec := xml.NewDecoder(strings.NewReader(string(out)))
		for {
			_, err := dec.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("Marshal produced XML that does not parse: %v\ninput: %q\noutput: %s",
					err, s, out)
			}
		}
	})
}
