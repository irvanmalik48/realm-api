package model

type Image struct {
	Size string `json:"size"`
	Text string `json:"#text"`
}

type TrackArtist struct {
	Mbid string `json:"mbid"`
	Text string `json:"#text"`
}

type TrackAlbum struct {
	Mbid string `json:"mbid"`
	Text string `json:"#text"`
}

type TrackAttr struct {
	NowPlaying string `json:"nowplaying"`
}

type TrackDate struct {
	Uts  string `json:"uts"`
	Text string `json:"#text"`
}

type Track struct {
	Attr       *TrackAttr  `json:"@attr,omitempty"`
	Date       *TrackDate  `json:"date,omitempty"`
	Artist     TrackArtist `json:"artist"`
	Album      TrackAlbum  `json:"album"`
	Mbid       string      `json:"mbid"`
	Streamable string      `json:"streamable"`
	Url        string      `json:"url"`
	Name       string      `json:"name"`
	Image      []Image     `json:"image"`
}

type RecentTracksAttr struct {
	Page       string `json:"page"`
	Total      string `json:"total"`
	User       string `json:"user"`
	PerPage    string `json:"perPage"`
	TotalPages string `json:"totalPages"`
}

type RecentTracks struct {
	Attr  RecentTracksAttr `json:"@attr"`
	Track []Track          `json:"track,omitempty"`
}

type LastFMTrackResponseBody struct {
	RecentTracks RecentTracks `json:"recenttracks"`
}

type LastFMUserRegistered struct {
	Text     interface{} `json:"#text"`
	UnixTime string      `json:"unixtime"`
}

type LastFMUser struct {
	Registered  LastFMUserRegistered `json:"registered"`
	TrackCount  string               `json:"track_count"`
	Age         string               `json:"age"`
	RealName    string               `json:"realname"`
	Bootstrap   string               `json:"bootstrap"`
	PlayCount   string               `json:"playcount"`
	ArtistCount string               `json:"artist_count"`
	Subscriber  string               `json:"subscriber"`
	AlbumCount  string               `json:"album_count"`
	Playlists   string               `json:"playlists"`
	Type        string               `json:"type"`
	Name        string               `json:"name"`
	Country     string               `json:"country"`
	Gender      string               `json:"gender"`
	Url         string               `json:"url"`
	Image       []Image              `json:"image"`
}

type LastFMUserResponseBody struct {
	User LastFMUser `json:"user"`
}
