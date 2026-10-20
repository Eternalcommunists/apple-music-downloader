package app

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"amdl/internal/amp-api"
	"amdl/internal/model"
)

// ProbeResult describes a URL the user pasted, together with the metadata the
// GUI shows before the download starts.
type ProbeResult struct {
	Kind        string       `json:"kind"`        // album | song | playlist | station | mv | artist
	URL         string       `json:"url"`         // normalized URL
	Storefront  string       `json:"storefront"`  // e.g. "us"
	ID          string       `json:"id"`          // catalog id
	Name        string       `json:"name"`        // album/song/playlist name
	Artist      string       `json:"artist"`      // primary artist
	ArtworkURL  string       `json:"artworkUrl"`  // cover art, {w}x{h} placeholder intact
	ReleaseDate string       `json:"releaseDate"` // ISO date when known
	TrackCount  int          `json:"trackCount"`  // total tracks when known
	Genres      []string     `json:"genres"`      // genre names
	RecordLabel string       `json:"recordLabel"` // when known
	Copyright   string       `json:"copyright"`   // when known
	UPC         string       `json:"upc"`         // when known
	IsMaster    bool         `json:"isMaster"`    // Apple Digital Master
	HasExplicit bool         `json:"hasExplicit"` // any explicit track
	Traits      []string     `json:"traits"`      // e.g. hi-res-lossless
	Tracks      []ProbeTrack `json:"tracks"`      // per-track preview list
	Qualities   []Quality    `json:"qualities"`   // downloadable qualities
	HasSongPick bool         `json:"hasSongPick"` // album URL carried ?i=<song>
}

// ProbeTrack is one row of the track list shown in the GUI.
type ProbeTrack struct {
	Number   int    `json:"number"`
	Disc     int    `json:"disc"`
	Name     string `json:"name"`
	Duration int    `json:"durationMillis"`
	Explicit bool   `json:"explicit"`
	// PreviewURL is Apple's own short preview clip, which is what in-page
	// playback streams. It is not the encrypted full track.
	PreviewURL string `json:"previewUrl"`
}

// Quality is one selectable download quality, already checked against what the
// title actually offers so the GUI never offers an unavailable option.
type Quality struct {
	ID          string `json:"id"`          // alac | aac | atmos | ac3
	Name        string `json:"name"`        // display name
	Available   bool   `json:"available"`   // false => greyed out in the GUI
	Reason      string `json:"reason"`      // why it is unavailable
	Description string `json:"description"` // detail line, e.g. "24bit/192kHz"
}

// probeURL reads a pasted URL and returns everything the GUI needs to render
// the confirmation screen: metadata, track list, and the qualities that are
// actually downloadable for this title.
func (r *Runner) probeURL(rawURL string, token string) (*ProbeResult, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errors.New("请粘贴 Apple Music 链接")
	}

	kind := detectURLKind(rawURL)
	if kind == "" {
		return nil, errors.New("不支持的链接：需要专辑、单曲、播放列表、电台、音乐视频或歌手页面的 Apple Music 链接")
	}

	res := &ProbeResult{Kind: kind, URL: rawURL, HasSongPick: parsedQuery(rawURL) != ""}

	switch kind {
	case "album":
		res.Storefront, res.ID = checkUrl(rawURL, "album")
		album := model.NewAlbum(res.Storefront, res.ID)
		if err := album.GetResp(token, r.Config.General.Language); err != nil {
			return nil, fmt.Errorf("获取专辑信息失败: %w", err)
		}
		fillFromAlbum(res, album)

	case "song":
		res.Storefront, res.ID = checkUrl(rawURL, "song")
		manifest, err := ampapi.GetSongResp(res.Storefront, res.ID, r.Config.General.Language, token)
		if err != nil {
			return nil, fmt.Errorf("获取单曲信息失败: %w", err)
		}
		if len(manifest.Data) == 0 {
			return nil, errors.New("未找到该单曲")
		}
		fillFromSong(res, manifest.Data[0])

	case "playlist":
		res.Storefront, res.ID = checkUrl(rawURL, "playlist")
		pl := model.NewPlaylist(res.Storefront, res.ID)
		if err := pl.GetResp(token, r.Config.General.Language); err != nil {
			return nil, fmt.Errorf("获取播放列表信息失败: %w", err)
		}
		fillFromPlaylist(res, pl)

	case "mv":
		res.Storefront, res.ID = checkUrl(rawURL, "mv")
		mv, err := ampapi.GetMusicVideoResp(res.Storefront, res.ID, r.Config.General.Language, token)
		if err != nil {
			return nil, fmt.Errorf("获取音乐视频信息失败: %w", err)
		}
		if len(mv.Data) == 0 {
			return nil, errors.New("未找到该音乐视频")
		}
		fillFromMusicVideo(res, mv.Data[0])

	case "station":
		res.Storefront, res.ID = checkUrl(rawURL, "station")
		res.Name = "Apple Music Station"
		res.Artist = "Apple Music Station"

	case "artist":
		res.Storefront, res.ID = checkUrl(rawURL, "artist")
		name, artistID, err := r.getUrlArtistName(rawURL, token)
		if err != nil {
			return nil, fmt.Errorf("获取歌手信息失败: %w", err)
		}
		res.Name = name
		res.Artist = name
		if res.ID == "" {
			res.ID = artistID
		}
	}

	res.Qualities = r.probeQualities(res)
	return res, nil
}

// detectURLKind returns the media kind for an Apple Music URL, or "" when the
// URL is not one this tool can handle. Music video is tested first because some
// MV links also contain an album path segment.
func detectURLKind(rawURL string) string {
	for _, kind := range []string{"mv", "song", "album", "playlist", "station", "artist"} {
		if _, id := checkUrl(rawURL, kind); id != "" {
			return kind
		}
	}
	return ""
}

func fillFromAlbum(res *ProbeResult, album *model.Album) {
	if len(album.Resp.Data) == 0 {
		return
	}
	meta := album.Resp.Data[0]
	res.Name = meta.Attributes.Name
	res.Artist = meta.Attributes.ArtistName
	res.ArtworkURL = meta.Attributes.Artwork.URL
	res.ReleaseDate = meta.Attributes.ReleaseDate
	res.TrackCount = meta.Attributes.TrackCount
	res.Genres = meta.Attributes.GenreNames
	res.RecordLabel = meta.Attributes.RecordLabel
	res.Copyright = meta.Attributes.Copyright
	res.UPC = meta.Attributes.Upc
	res.IsMaster = meta.Attributes.IsAppleDigitalMaster || meta.Attributes.IsMasteredForItunes
	res.Traits = meta.Attributes.AudioTraits
	if res.TrackCount == 0 {
		res.TrackCount = len(meta.Relationships.Tracks.Data)
	}
	res.Tracks = make([]ProbeTrack, 0, len(meta.Relationships.Tracks.Data))
	for _, t := range meta.Relationships.Tracks.Data {
		tr := ProbeTrack{
			Number:     t.Attributes.TrackNumber,
			Disc:       t.Attributes.DiscNumber,
			Name:       t.Attributes.Name,
			Duration:   t.Attributes.DurationInMillis,
			PreviewURL: firstPreviewURL(t.Attributes.Previews),
		}
		if t.Attributes.ContentRating == "explicit" {
			tr.Explicit = true
			res.HasExplicit = true
		}
		res.Tracks = append(res.Tracks, tr)
	}
}

func fillFromSong(res *ProbeResult, song ampapi.SongRespData) {
	res.Name = song.Attributes.Name
	res.Artist = song.Attributes.ArtistName
	res.ArtworkURL = song.Attributes.Artwork.URL
	res.ReleaseDate = song.Attributes.ReleaseDate
	res.TrackCount = 1
	res.Genres = song.Attributes.GenreNames
	res.IsMaster = song.Attributes.IsAppleDigitalMaster || song.Attributes.IsMasteredForItunes
	res.Traits = song.Attributes.AudioTraits
	res.HasExplicit = song.Attributes.ContentRating == "explicit"
	res.Tracks = []ProbeTrack{{
		Number:     song.Attributes.TrackNumber,
		Disc:       song.Attributes.DiscNumber,
		Name:       song.Attributes.Name,
		Duration:   song.Attributes.DurationInMillis,
		Explicit:   res.HasExplicit,
		PreviewURL: firstPreviewURL(song.Attributes.Previews),
	}}
}

// firstPreviewURL returns the first preview clip URL, or "" when the title has
// no preview. Apple normally publishes exactly one preview per track.
func firstPreviewURL(previews []struct {
	URL string `json:"url"`
}) string {
	if len(previews) == 0 {
		return ""
	}
	return previews[0].URL
}

func fillFromPlaylist(res *ProbeResult, pl *model.Playlist) {
	if len(pl.Resp.Data) == 0 {
		return
	}
	meta := pl.Resp.Data[0]
	res.Name = meta.Attributes.Name
	res.Artist = "Apple Music"
	res.ArtworkURL = meta.Attributes.Artwork.URL
	res.TrackCount = len(meta.Relationships.Tracks.Data)
	res.Tracks = make([]ProbeTrack, 0, res.TrackCount)
	for _, t := range meta.Relationships.Tracks.Data {
		res.Tracks = append(res.Tracks, ProbeTrack{
			Number:   t.Attributes.TrackNumber,
			Disc:     t.Attributes.DiscNumber,
			Name:     t.Attributes.Name,
			Duration: t.Attributes.DurationInMillis,
		})
	}
}

func fillFromMusicVideo(res *ProbeResult, mv ampapi.MusicVideoRespData) {
	res.Name = mv.Attributes.Name
	res.Artist = mv.Attributes.ArtistName
	res.ArtworkURL = mv.Attributes.Artwork.URL
	res.ReleaseDate = mv.Attributes.ReleaseDate
	res.TrackCount = 1
	res.Genres = mv.Attributes.GenreNames
}

// probeQualities decides which qualities the GUI may offer, based on the audio
// traits the catalog reports for the title.
func (r *Runner) probeQualities(res *ProbeResult) []Quality {
	if res.Kind == "artist" {
		// An artist expands into many albums; quality is chosen per album.
		return nil
	}

	if res.Kind == "mv" {
		return []Quality{
			{ID: "atmos", Name: "杜比全景声（MV 音轨）", Available: true, Description: "E-AC-3 16 声道"},
			{ID: "ac3", Name: "杜比音效（MV 音轨）", Available: true, Description: "AC-3 16 声道"},
			{ID: "aac", Name: "AAC（MV 音轨）", Available: true, Description: "立体声"},
		}
	}

	hasHiRes := contains(res.Traits, "hi-res-lossless")
	hasLossless := hasHiRes || contains(res.Traits, "lossless")
	hasAtmos := contains(res.Traits, "atmos")

	alacReason := ""
	if !hasLossless {
		alacReason = "该内容未提供无损音质"
	}
	alacDesc := "CD 音质或更高"
	if hasHiRes {
		alacDesc = "高解析无损，最高 24bit/192kHz"
	} else if hasLossless {
		alacDesc = "无损，最高 24bit/48kHz"
	}

	atmosReason := ""
	if !hasAtmos {
		atmosReason = "该内容未提供杜比全景声"
	}

	return []Quality{
		{ID: "alac", Name: "无损 (ALAC)", Available: alacReason == "", Reason: alacReason, Description: alacDesc},
		{ID: "aac", Name: "高品质 (AAC)", Available: true, Description: "256 kbps 立体声"},
		{ID: "atmos", Name: "杜比全景声", Available: atmosReason == "", Reason: atmosReason, Description: "E-AC-3 空间音频"},
	}
}

// applyQualityFlavor maps a GUI quality id onto the runner flags the download
// engine already understands.
func (r *Runner) applyQualityFlavor(quality string) error {
	switch quality {
	case "", "alac":
		r.Flags.Atmos = false
		r.Flags.AAC = false
	case "aac":
		r.Flags.Atmos = false
		r.Flags.AAC = true
		r.Config.Media.AacType = "aac"
	case "atmos":
		r.Flags.Atmos = true
		r.Flags.AAC = false
	case "ac3":
		// AC-3 is a music-video audio option, not an audio quality.
		r.Config.Media.MV.AudioType = "ac3"
	default:
		return fmt.Errorf("未知音质: %s", quality)
	}
	return nil
}

// parsedQuery returns the ?i= song id from an album URL, if present.
func parsedQuery(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Query().Get("i")
}
