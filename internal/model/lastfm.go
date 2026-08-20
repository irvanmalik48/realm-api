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
	Artist     TrackArtist `json:"artist"`
	Attr       *TrackAttr  `json:"@attr,omitempty"`
	Mbid       string      `json:"mbid"`
	Album      TrackAlbum  `json:"album"`
	Streamable string      `json:"streamable"`
	Url        string      `json:"url"`
	Name       string      `json:"name"`
	Image      []Image     `json:"image"`
	Date       *TrackDate  `json:"date,omitempty"`
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
	UnixTime string      `json:"unixtime"`
	Text     interface{} `json:"#text"`
}

type LastFMUser struct {
	Name        string               `json:"name"`
	Age         string               `json:"age"`
	Subscriber  string               `json:"subscriber"`
	RealName    string               `json:"realname"`
	Bootstrap   string               `json:"bootstrap"`
	PlayCount   string               `json:"playcount"`
	ArtistCount string               `json:"artist_count"`
	Playlists   string               `json:"playlists"`
	TrackCount  string               `json:"track_count"`
	AlbumCount  string               `json:"album_count"`
	Image       []Image              `json:"image"`
	Registered  LastFMUserRegistered `json:"registered"`
	Country     string               `json:"country"`
	Gender      string               `json:"gender"`
	Url         string               `json:"url"`
	Type        string               `json:"type"`
}

type LastFMUserResponseBody struct {
	User LastFMUser `json:"user"`
}
