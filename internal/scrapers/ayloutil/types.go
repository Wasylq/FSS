package ayloutil

import "encoding/json"

type ReleasesResponse struct {
	Meta   APIMeta   `json:"meta"`
	Result []Release `json:"result"`
}

type APIMeta struct {
	Count int `json:"count"`
	Total int `json:"total"`
}

type Release struct {
	ID           int             `json:"id"`
	Type         string          `json:"type"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	DateReleased string          `json:"dateReleased"`
	IsVR         bool            `json:"isVR"`
	Actors       []Actor         `json:"actors"`
	Collections  []Collection    `json:"collections"`
	Tags         []Tag           `json:"tags"`
	Stats        Stats           `json:"stats"`
	Groups       []Group         `json:"groups"`
	Children     []Release       `json:"children"`
	RawImages    json.RawMessage `json:"images"`
	RawVideos    json.RawMessage `json:"videos"`
}

type Actor struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
}

type Collection struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"shortName"`
}

type Tag struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type Stats struct {
	Likes     int `json:"likes"`
	Dislikes  int `json:"dislikes"`
	Rating    int `json:"rating"`
	Views     int `json:"views"`
	Downloads int `json:"downloads"`
}

type Group struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}
