package stash

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
)

type Client struct {
	url    string
	apiKey string
	http   *http.Client
}

func NewClient(url, apiKey string) *Client {
	graphqlURL := url + "/graphql"
	return &Client{
		url:    graphqlURL,
		apiKey: apiKey,
		http:   httpx.NewClient(30 * time.Second),
	}
}

// StashScene represents a scene as returned by Stash's findScenes query.
type StashScene struct {
	ID         string       `json:"id"`
	Title      string       `json:"title"`
	Date       string       `json:"date"`
	Details    string       `json:"details"`
	URLs       []string     `json:"urls"`
	Organized  bool         `json:"organized"`
	Files      []StashFile  `json:"files"`
	Tags       []StashTag   `json:"tags"`
	Performers []StashPerf  `json:"performers"`
	Studio     *StashStudio `json:"studio"`
	StashIDs   []StashID    `json:"stash_ids"`
}

type StashFile struct {
	Basename string  `json:"basename"`
	Path     string  `json:"path"`
	Duration float64 `json:"duration"`
}

type StashTag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type StashPerf struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type StashStudio struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type StashID struct {
	Endpoint string `json:"endpoint"`
	StashID  string `json:"stash_id"`
}

type SceneUpdateInput struct {
	ID          string   `json:"id"`
	Title       *string  `json:"title,omitempty"`
	Details     *string  `json:"details,omitempty"`
	Date        *string  `json:"date,omitempty"`
	URLs        []string `json:"urls,omitempty"`
	TagIDs      []string `json:"tag_ids,omitempty"`
	PerformerIDs []string `json:"performer_ids,omitempty"`
	StudioID    *string  `json:"studio_id,omitempty"`
	Organized   *bool    `json:"organized,omitempty"`
}

type ScrapedScene struct {
	Title       *string   `json:"title"`
	Details     *string   `json:"details"`
	Date        *string   `json:"date"`
	URLs        []string  `json:"urls"`
	Tags        []ScrapedTag `json:"tags"`
	Performers  []ScrapedPerf `json:"performers"`
	Studio      *ScrapedStudio `json:"studio"`
}

type ScrapedTag struct {
	Name string `json:"name"`
}

type ScrapedPerf struct {
	Name string `json:"name"`
}

type ScrapedStudio struct {
	Name string `json:"name"`
}

type FindScenesFilter struct {
	Organized       *bool   `json:"organized,omitempty"`
	PerformerName   string  `json:"-"`
	StudioName      string  `json:"-"`
	StashIDCount    *int    `json:"-"`
}

type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *Client) do(ctx context.Context, gql graphqlRequest) (json.RawMessage, error) {
	body, err := json.Marshal(gql)
	if err != nil {
		return nil, fmt.Errorf("marshalling graphql request: %w", err)
	}
	headers := map[string]string{"Content-Type": "application/json"}
	if c.apiKey != "" {
		headers["ApiKey"] = c.apiKey
	}

	resp, err := httpx.Do(ctx, c.http, httpx.Request{
		Method:  http.MethodPost,
		URL:     c.url,
		Body:    body,
		Headers: headers,
	})
	if err != nil {
		return nil, fmt.Errorf("stash graphql: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading stash response: %w", err)
	}

	var gqlResp graphqlResponse
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return nil, fmt.Errorf("parsing stash response: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("stash api: %s", gqlResp.Errors[0].Message)
	}
	return gqlResp.Data, nil
}

// Ping validates the connection to Stash.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, graphqlRequest{Query: `{ systemStatus { status } }`})
	return err
}

const findScenesQuery = `
query FindScenes($filter: FindFilterType, $scene_filter: SceneFilterType) {
  findScenes(filter: $filter, scene_filter: $scene_filter) {
    count
    scenes {
      id
      title
      date
      details
      urls
      organized
      files { basename path duration }
      tags { id name }
      performers { id name }
      studio { id name }
      stash_ids { endpoint stash_id }
    }
  }
}`

func (c *Client) FindScenes(ctx context.Context, filter FindScenesFilter, page, perPage int) ([]StashScene, int, error) {
	sceneFilter := map[string]any{}
	findFilter := map[string]any{
		"page":     page,
		"per_page": perPage,
		"sort":     "path",
		"direction": "ASC",
	}

	if filter.StashIDCount != nil {
		sceneFilter["stash_id_endpoint"] = map[string]any{
			"modifier": "NOT_NULL",
		}
		if *filter.StashIDCount == 0 {
			sceneFilter["stash_id_endpoint"] = map[string]any{
				"modifier": "IS_NULL",
			}
		}
	}

	if filter.PerformerName != "" {
		sceneFilter["performers"] = map[string]any{
			"value":    []string{},
			"modifier": "INCLUDES_ALL",
		}
		perfID, found, err := c.FindPerformerByName(ctx, filter.PerformerName)
		if err != nil {
			return nil, 0, fmt.Errorf("looking up performer %q: %w", filter.PerformerName, err)
		}
		if !found {
			return nil, 0, nil
		}
		sceneFilter["performers"] = map[string]any{
			"value":    []string{perfID},
			"modifier": "INCLUDES_ALL",
		}
	}

	if filter.StudioName != "" {
		studioID, found, err := c.FindStudioByName(ctx, filter.StudioName)
		if err != nil {
			return nil, 0, fmt.Errorf("looking up studio %q: %w", filter.StudioName, err)
		}
		if !found {
			return nil, 0, nil
		}
		sceneFilter["studios"] = map[string]any{
			"value":    []string{studioID},
			"modifier": "INCLUDES_ALL",
			"depth":    0,
		}
	}

	vars := map[string]any{"filter": findFilter}
	if len(sceneFilter) > 0 {
		vars["scene_filter"] = sceneFilter
	}

	data, err := c.do(ctx, graphqlRequest{
		Query:     findScenesQuery,
		Variables: vars,
	})
	if err != nil {
		return nil, 0, err
	}

	var result struct {
		FindScenes struct {
			Count  int          `json:"count"`
			Scenes []StashScene `json:"scenes"`
		} `json:"findScenes"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, 0, fmt.Errorf("parsing findScenes: %w", err)
	}
	return result.FindScenes.Scenes, result.FindScenes.Count, nil
}

// ProgressFunc is called after each page is fetched during FindAllScenes.
// fetched is the running total, total is the count from the first page.
type ProgressFunc func(fetched, total int)

// FindAllScenes paginates through all matching scenes.
// If progress is non-nil it is called after each page with the running count.
func (c *Client) FindAllScenes(ctx context.Context, filter FindScenesFilter, progress ProgressFunc) ([]StashScene, error) {
	const perPage = 100
	var all []StashScene
	var total int
	page := 1
	for {
		scenes, count, err := c.FindScenes(ctx, filter, page, perPage)
		if err != nil {
			return nil, err
		}
		if page == 1 {
			total = count
		}
		all = append(all, scenes...)
		if progress != nil {
			progress(len(all), total)
		}
		if len(scenes) < perPage {
			break
		}
		page++
	}
	return all, nil
}

func (c *Client) FindTagByName(ctx context.Context, name string) (string, bool, error) {
	data, err := c.do(ctx, graphqlRequest{
		Query: `query($name: String!) { findTags(tag_filter: { name: { value: $name, modifier: EQUALS } }) { tags { id name } } }`,
		Variables: map[string]any{"name": name},
	})
	if err != nil {
		return "", false, err
	}
	var result struct {
		FindTags struct {
			Tags []StashTag `json:"tags"`
		} `json:"findTags"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", false, err
	}
	if len(result.FindTags.Tags) == 0 {
		return "", false, nil
	}
	return result.FindTags.Tags[0].ID, true, nil
}

func (c *Client) CreateTag(ctx context.Context, name string) (string, error) {
	data, err := c.do(ctx, graphqlRequest{
		Query: `mutation($input: TagCreateInput!) { tagCreate(input: $input) { id } }`,
		Variables: map[string]any{"input": map[string]any{"name": name}},
	})
	if err != nil {
		return "", err
	}
	var result struct {
		TagCreate struct{ ID string `json:"id"` } `json:"tagCreate"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.TagCreate.ID, nil
}

func (c *Client) EnsureTag(ctx context.Context, name string) (string, error) {
	id, found, err := c.FindTagByName(ctx, name)
	if err != nil {
		return "", err
	}
	if found {
		return id, nil
	}
	return c.CreateTag(ctx, name)
}

func (c *Client) FindPerformerByName(ctx context.Context, name string) (string, bool, error) {
	data, err := c.do(ctx, graphqlRequest{
		Query: `query($name: String!) { findPerformers(performer_filter: { name: { value: $name, modifier: EQUALS } }) { performers { id name } } }`,
		Variables: map[string]any{"name": name},
	})
	if err != nil {
		return "", false, err
	}
	var result struct {
		FindPerformers struct {
			Performers []StashPerf `json:"performers"`
		} `json:"findPerformers"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", false, err
	}
	if len(result.FindPerformers.Performers) == 0 {
		return "", false, nil
	}
	return result.FindPerformers.Performers[0].ID, true, nil
}

func (c *Client) CreatePerformer(ctx context.Context, name string) (string, error) {
	data, err := c.do(ctx, graphqlRequest{
		Query: `mutation($input: PerformerCreateInput!) { performerCreate(input: $input) { id } }`,
		Variables: map[string]any{"input": map[string]any{"name": name}},
	})
	if err != nil {
		return "", err
	}
	var result struct {
		PerformerCreate struct{ ID string `json:"id"` } `json:"performerCreate"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.PerformerCreate.ID, nil
}

func (c *Client) EnsurePerformer(ctx context.Context, name string) (string, error) {
	id, found, err := c.FindPerformerByName(ctx, name)
	if err != nil {
		return "", err
	}
	if found {
		return id, nil
	}
	return c.CreatePerformer(ctx, name)
}

func (c *Client) FindStudioByName(ctx context.Context, name string) (string, bool, error) {
	data, err := c.do(ctx, graphqlRequest{
		Query: `query($name: String!) { findStudios(studio_filter: { name: { value: $name, modifier: EQUALS } }) { studios { id name } } }`,
		Variables: map[string]any{"name": name},
	})
	if err != nil {
		return "", false, err
	}
	var result struct {
		FindStudios struct {
			Studios []StashStudio `json:"studios"`
		} `json:"findStudios"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", false, err
	}
	if len(result.FindStudios.Studios) == 0 {
		return "", false, nil
	}
	return result.FindStudios.Studios[0].ID, true, nil
}

func (c *Client) CreateStudio(ctx context.Context, name string) (string, error) {
	data, err := c.do(ctx, graphqlRequest{
		Query: `mutation($input: StudioCreateInput!) { studioCreate(input: $input) { id } }`,
		Variables: map[string]any{"input": map[string]any{"name": name}},
	})
	if err != nil {
		return "", err
	}
	var result struct {
		StudioCreate struct{ ID string `json:"id"` } `json:"studioCreate"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.StudioCreate.ID, nil
}

func (c *Client) EnsureStudio(ctx context.Context, name string) (string, error) {
	id, found, err := c.FindStudioByName(ctx, name)
	if err != nil {
		return "", err
	}
	if found {
		return id, nil
	}
	return c.CreateStudio(ctx, name)
}

const sceneUpdateMutation = `
mutation SceneUpdate($input: SceneUpdateInput!) {
  sceneUpdate(input: $input) { id }
}`

func (c *Client) UpdateScene(ctx context.Context, input SceneUpdateInput) error {
	_, err := c.do(ctx, graphqlRequest{
		Query:     sceneUpdateMutation,
		Variables: map[string]any{"input": input},
	})
	return err
}

func (c *Client) ScrapeSceneURL(ctx context.Context, url string) (*ScrapedScene, error) {
	data, err := c.do(ctx, graphqlRequest{
		Query: `query($url: String!) { scrapeURL(url: $url, ty: SCENE) { ... on ScrapedScene { title details date urls tags { name } performers { name } studio { name } } } }`,
		Variables: map[string]any{"url": url},
	})
	if err != nil {
		return nil, err
	}
	var result struct {
		ScrapeURL *ScrapedScene `json:"scrapeURL"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.ScrapeURL, nil
}
