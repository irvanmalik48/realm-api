package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/service"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LastFMServer struct {
	realmv1.UnimplementedLastFMServiceServer
	cfg     *config.Config
	lastFMSvc service.LastFMService
}

func NewLastFMServer(cfg *config.Config, lastFMSvc service.LastFMService) *LastFMServer {
	return &LastFMServer{
		cfg:     cfg,
		lastFMSvc: lastFMSvc,
	}
}

func mapTrackImages(imgs []model.Image) []*realmv1.LastFMImage {
	var res []*realmv1.LastFMImage
	for _, img := range imgs {
		res = append(res, &realmv1.LastFMImage{
			Size: img.Size,
			Url:  img.Text,
		})
	}
	return res
}

func (s *LastFMServer) GetRecentTracks(ctx context.Context, req *realmv1.GetRecentTracksRequest) (*realmv1.RecentTracksResponse, error) {
	if s.cfg.LastFMAPIKey == "" {
		return nil, status.Error(codes.FailedPrecondition, "LastFM API is not configured on this server")
	}

	username := strings.TrimSpace(req.GetUsername())
	if username == "" {
		return nil, status.Error(codes.InvalidArgument, "Username is required")
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 1
	} else if limit > 200 {
		limit = 200
	}

	data, err := s.lastFMSvc.GetRecentTracks(ctx, username, limit)
	if err != nil {
		return nil, status.Error(codes.Unavailable, fmt.Sprintf("Failed to fetch LastFM tracks: %v", err))
	}

	rawBytes, _ := json.Marshal(data)

	var protoTracks []*realmv1.LastFMTrack
	for _, t := range data.RecentTracks.Track {
		nowPlaying := false
		if t.Attr != nil && t.Attr.NowPlaying == "true" {
			nowPlaying = true
		}

		var trackDate *realmv1.LastFMTrackDate
		if t.Date != nil {
			trackDate = &realmv1.LastFMTrackDate{
				Uts:  t.Date.Uts,
				Text: t.Date.Text,
			}
		}

		protoTracks = append(protoTracks, &realmv1.LastFMTrack{
			Name:       t.Name,
			Mbid:       t.Mbid,
			Url:        t.Url,
			Streamable: t.Streamable,
			Artist: &realmv1.LastFMTrackArtist{
				Mbid: t.Artist.Mbid,
				Name: t.Artist.Text,
			},
			Album: &realmv1.LastFMTrackAlbum{
				Mbid:  t.Album.Mbid,
				Title: t.Album.Text,
			},
			Images:     mapTrackImages(t.Image),
			Date:       trackDate,
			NowPlaying: nowPlaying,
		})
	}

	return &realmv1.RecentTracksResponse{
		Attr: &realmv1.LastFMRecentTracksAttr{
			Page:       data.RecentTracks.Attr.Page,
			Total:      data.RecentTracks.Attr.Total,
			User:       data.RecentTracks.Attr.User,
			PerPage:    data.RecentTracks.Attr.PerPage,
			TotalPages: data.RecentTracks.Attr.TotalPages,
		},
		Tracks:  protoTracks,
		RawJson: string(rawBytes),
	}, nil
}

func (s *LastFMServer) GetUserInfo(ctx context.Context, req *realmv1.GetUserInfoRequest) (*realmv1.UserInfoResponse, error) {
	if s.cfg.LastFMAPIKey == "" {
		return nil, status.Error(codes.FailedPrecondition, "LastFM API is not configured on this server")
	}

	username := strings.TrimSpace(req.GetUsername())
	if username == "" {
		return nil, status.Error(codes.InvalidArgument, "Username is required")
	}

	data, err := s.lastFMSvc.GetUserInfo(ctx, username)
	if err != nil {
		return nil, status.Error(codes.Unavailable, fmt.Sprintf("Failed to fetch LastFM user: %v", err))
	}

	rawBytes, _ := json.Marshal(data)
	u := data.User

	return &realmv1.UserInfoResponse{
		User: &realmv1.LastFMUser{
			Name:        u.Name,
			RealName:    u.RealName,
			Country:     u.Country,
			Age:         u.Age,
			Gender:      u.Gender,
			Subscriber:  u.Subscriber,
			PlayCount:   u.PlayCount,
			ArtistCount: u.ArtistCount,
			TrackCount:  u.TrackCount,
			AlbumCount:  u.AlbumCount,
			Playlists:   u.Playlists,
			Url:         u.Url,
			Type:        u.Type,
			Images:      mapTrackImages(u.Image),
			Registered: &realmv1.LastFMUserRegistered{
				UnixTime: u.Registered.UnixTime,
				Text:     fmt.Sprintf("%v", u.Registered.Text),
			},
		},
		RawJson: string(rawBytes),
	}, nil
}
