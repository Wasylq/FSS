package nfo

import (
	"encoding/xml"
	"time"

	"github.com/Wasylq/FSS/match"
)

// Movie is a Kodi-style NFO structure for a single video.
type Movie struct {
	XMLName    xml.Name `xml:"movie"`
	Title      string   `xml:"title"`
	URLs       []string `xml:"url,omitempty"`
	Premiered  string   `xml:"premiered,omitempty"`
	Plot       string   `xml:"plot,omitempty"`
	Studio     string   `xml:"studio,omitempty"`
	Thumbnails []Thumb  `xml:"thumb,omitempty"`
	Actors     []Actor  `xml:"actor,omitempty"`
	Tags       []string `xml:"tag,omitempty"`
}

// Thumb is a poster or fanart thumbnail URL with an aspect hint.
type Thumb struct {
	Aspect string `xml:"aspect,attr"`
	URL    string `xml:",chardata"`
}

// Actor is a performer credit in the NFO.
type Actor struct {
	Name string `xml:"name"`
}

// FromMergedScene converts a cross-site merged scene into a Kodi NFO Movie.
func FromMergedScene(m match.MergedScene) Movie {
	mov := Movie{
		Title:  m.Title,
		URLs:   m.URLs,
		Plot:   m.Description,
		Studio: m.Studio,
		Tags:   m.Tags,
	}

	if !m.Date.IsZero() && m.Date.Year() > 1 {
		mov.Premiered = m.Date.Format(time.DateOnly)
	}

	if m.Thumbnail != "" {
		mov.Thumbnails = []Thumb{{Aspect: "poster", URL: m.Thumbnail}}
	}

	for _, p := range m.Performers {
		mov.Actors = append(mov.Actors, Actor{Name: p})
	}

	return mov
}

// Marshal serialises a Movie to XML with an XML declaration header.
func Marshal(m Movie) ([]byte, error) {
	body, err := xml.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}
