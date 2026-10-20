package app

import (
	"errors"
	"fmt"
	"strings"

	"amdl/internal/amp-api"
)

// SearchHit is one row in the GUI search results. Clicking a hit hands its URL
// to the normal probe flow, so the user goes straight from search to the
// download confirmation screen without pasting a link.
type SearchHit struct {
	Kind        string   `json:"kind"` // album | song | artist
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Artist      string   `json:"artist"`
	AlbumName   string   `json:"albumName"`   // songs only
	ReleaseDate string   `json:"releaseDate"` // albums only
	TrackCount  int      `json:"trackCount"`  // albums only
	Genres      []string `json:"genres"`      // artists only
	Duration    int      `json:"durationMillis"`
	Explicit    bool     `json:"explicit"`
	ArtworkURL  string   `json:"artworkUrl"`
	URL         string   `json:"url"` // Apple Music URL, fed straight into /api/probe
}

// SearchResponse is the payload the GUI renders for one search page.
type SearchResponse struct {
	Query   string      `json:"query"`
	Type    string      `json:"type"`
	Offset  int         `json:"offset"`
	Limit   int         `json:"limit"`
	HasMore bool        `json:"hasMore"`
	Hits    []SearchHit `json:"hits"`
}

// searchLimit is how many results one page returns. It mirrors the CLI's page
// size so both front ends surface a comparable amount of data.
const searchLimit = 15

// searchKinds maps the GUI's type selector onto the API's `types` parameter.
var searchKinds = map[string]string{
	"album":  "albums",
	"song":   "songs",
	"artist": "artists",
}

// searchURL builds the canonical Apple Music URL for a hit. The catalog search
// response carries hrefs, but they are API paths rather than the user-facing
// web URLs the probe expects, so the link is assembled from the ids instead.
func searchURL(storefront, kind, id string, songID string) string {
	sf := storefront
	if sf == "" {
		sf = "us"
	}
	switch kind {
	case "album":
		return fmt.Sprintf("https://music.apple.com/%s/album/%s", sf, id)
	case "song":
		// A song is reachable through its album with the ?i= selector, which is
		// also the form the CLI downloads single songs with.
		if songID != "" {
			return fmt.Sprintf("https://music.apple.com/%s/album/%s?i=%s", sf, songID, id)
		}
		return fmt.Sprintf("https://music.apple.com/%s/song/%s", sf, id)
	case "artist":
		return fmt.Sprintf("https://music.apple.com/%s/artist/%s", sf, id)
	default:
		return ""
	}
}

// searchAppleMusic runs one search page and maps it into GUI-friendly hits.
// It validates the requested type rather than passing an arbitrary string to
// the API, so the endpoint cannot be used to probe other resource types.
func (r *Runner) searchAppleMusic(query, kind string, offset int) (*SearchResponse, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("请输入搜索关键词")
	}

	apiTypes, ok := searchKinds[kind]
	if !ok {
		return nil, fmt.Errorf("不支持的搜索类型: %s（可选 album / song / artist）", kind)
	}
	if offset < 0 {
		offset = 0
	}

	resp, err := ampapi.Search(
		r.Config.General.Storefront,
		query,
		apiTypes,
		r.Config.General.Language,
		r.Token,
		searchLimit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}

	out := &SearchResponse{
		Query:  query,
		Type:   kind,
		Offset: offset,
		Limit:  searchLimit,
		Hits:   []SearchHit{},
	}

	switch kind {
	case "album":
		if resp.Results.Albums != nil {
			for _, item := range resp.Results.Albums.Data {
				out.Hits = append(out.Hits, SearchHit{
					Kind:        "album",
					ID:          item.ID,
					Name:        item.Attributes.Name,
					Artist:      item.Attributes.ArtistName,
					ReleaseDate: item.Attributes.ReleaseDate,
					TrackCount:  item.Attributes.TrackCount,
					Explicit:    item.Attributes.ContentRating == "explicit",
					ArtworkURL:  item.Attributes.Artwork.URL,
					URL:         searchURL(r.Config.General.Storefront, "album", item.ID, ""),
				})
			}
			out.HasMore = resp.Results.Albums.Next != ""
		}

	case "song":
		if resp.Results.Songs != nil {
			for _, item := range resp.Results.Songs.Data {
				// Prefer the album relationship: it yields a URL the probe can
				// resolve into a full album context for a single song.
				albumID := ""
				if len(item.Relationships.Albums.Data) > 0 {
					albumID = item.Relationships.Albums.Data[0].ID
				}
				out.Hits = append(out.Hits, SearchHit{
					Kind:       "song",
					ID:         item.ID,
					Name:       item.Attributes.Name,
					Artist:     item.Attributes.ArtistName,
					AlbumName:  item.Attributes.AlbumName,
					Duration:   item.Attributes.DurationInMillis,
					Explicit:   item.Attributes.ContentRating == "explicit",
					ArtworkURL: item.Attributes.Artwork.URL,
					URL:        searchURL(r.Config.General.Storefront, "song", item.ID, albumID),
				})
			}
			out.HasMore = resp.Results.Songs.Next != ""
		}

	case "artist":
		if resp.Results.Artists != nil {
			for _, item := range resp.Results.Artists.Data {
				out.Hits = append(out.Hits, SearchHit{
					Kind:   "artist",
					ID:     item.ID,
					Name:   item.Attributes.Name,
					Artist: item.Attributes.Name,
					Genres: item.Attributes.GenreNames,
					URL:    searchURL(r.Config.General.Storefront, "artist", item.ID, ""),
				})
			}
			out.HasMore = resp.Results.Artists.Next != ""
		}
	}

	return out, nil
}
