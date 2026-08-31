package server

import (
	"context"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/grpc/interceptors"
	"github.com/irvanmalik48/realm-api/internal/service"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ReactionServer struct {
	realmv1.UnimplementedReactionServiceServer
	reactionSvc service.ReactionService
}

func NewReactionServer(reactionSvc service.ReactionService) *ReactionServer {
	return &ReactionServer{reactionSvc: reactionSvc}
}

func (s *ReactionServer) GetReactions(ctx context.Context, req *realmv1.GetReactionsRequest) (*realmv1.ReactionsResponse, error) {
	slug := req.GetSlug()
	if slug == "" {
		return nil, status.Error(codes.InvalidArgument, "Slug is required")
	}

	var userIDPtr *uuid.UUID
	if id, ok := interceptors.GetUserID(ctx); ok {
		userIDPtr = &id
	}

	resp, err := s.reactionSvc.GetReactions(ctx, slug, userIDPtr)
	if err != nil {
		return nil, err
	}

	reactionsMap := make(map[string]int32)
	for k, v := range resp.Reactions {
		reactionsMap[k] = int32(v)
	}

	return &realmv1.ReactionsResponse{
		Slug:          resp.Slug,
		TotalCount:    int32(resp.TotalCount),
		Reactions:     reactionsMap,
		UserReaction:  resp.UserReaction,
		UserReactions: resp.UserReactions,
	}, nil
}

func (s *ReactionServer) ToggleReaction(ctx context.Context, req *realmv1.ToggleReactionRequest) (*realmv1.ToggleReactionResponse, error) {
	userID, err := interceptors.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}

	slug := req.GetSlug()
	if slug == "" {
		return nil, status.Error(codes.InvalidArgument, "Slug is required")
	}

	reaction := req.GetReaction()
	if reaction == "" {
		return nil, status.Error(codes.InvalidArgument, "Reaction is required")
	}

	resp, err := s.reactionSvc.ToggleReaction(ctx, slug, reaction, userID)
	if err != nil {
		return nil, err
	}

	reactionsMap := make(map[string]int32)
	for k, v := range resp.Reactions {
		reactionsMap[k] = int32(v)
	}

	return &realmv1.ToggleReactionResponse{
		Slug:          resp.Slug,
		Reaction:      resp.Reaction,
		Active:        resp.Active,
		TotalCount:    int32(resp.TotalCount),
		Reactions:     reactionsMap,
		UserReaction:  resp.UserReaction,
		UserReactions: resp.UserReactions,
	}, nil
}
