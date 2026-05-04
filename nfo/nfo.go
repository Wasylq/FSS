package nfo

import (
	"encoding/xml"
	"time"

	"github.com/Wasylq/FSS/match"
)

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

type Thumb struct {
	Aspect string `xml:"aspect,attr"`
	URL    string `xml:",chardata"`
}

type Actor struct {
	Name string `xml:"name"`
}

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

func Marshal(m Movie) ([]byte, error) {
	body, err := xml.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}
