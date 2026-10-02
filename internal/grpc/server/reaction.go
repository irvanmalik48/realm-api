package server

import (
	"context"
	"math"

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
		if v >= 0 && v <= math.MaxInt32 {
			reactionsMap[k] = int32(v)
		}
	}

	var totalCount int32
	if resp.TotalCount >= 0 && resp.TotalCount <= math.MaxInt32 {
		totalCount = int32(resp.TotalCount)
	}

	return &realmv1.ReactionsResponse{
		Slug:          resp.Slug,
		TotalCount:    totalCount,
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
		if v >= 0 && v <= math.MaxInt32 {
			reactionsMap[k] = int32(v)
		}
	}

	var totalCount int32
	if resp.TotalCount >= 0 && resp.TotalCount <= math.MaxInt32 {
		totalCount = int32(resp.TotalCount)
	}

	return &realmv1.ToggleReactionResponse{
		Slug:          resp.Slug,
		Reaction:      resp.Reaction,
		Active:        resp.Active,
		TotalCount:    totalCount,
		Reactions:     reactionsMap,
		UserReaction:  resp.UserReaction,
		UserReactions: resp.UserReactions,
	}, nil
}

func (s *ReactionServer) GetReactionsSummary(ctx context.Context, req *realmv1.GetReactionsSummaryRequest) (*realmv1.GetReactionsSummaryResponse, error) {
	summaries, err := s.reactionSvc.GetSummaries(ctx, int(req.GetLimit()), int(req.GetOffset()), req.GetSearch())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get reactions summary: %v", err)
	}

	pbSummaries := make([]*realmv1.ReactionSummary, 0, len(summaries))
	for _, sum := range summaries {
		rMap := make(map[string]int32)
		for k, v := range sum.Reactions {
			rMap[k] = int32(v)
		}
		pbSummaries = append(pbSummaries, &realmv1.ReactionSummary{
			Slug:       sum.Slug,
			TotalCount: int32(sum.TotalCount),
			Reactions:  rMap,
		})
	}

	return &realmv1.GetReactionsSummaryResponse{
		Summaries: pbSummaries,
	}, nil
}

func (s *ReactionServer) DeleteReaction(ctx context.Context, req *realmv1.DeleteReactionRequest) (*realmv1.DeleteReactionResponse, error) {
	slug := req.GetSlug()
	if slug == "" {
		return nil, status.Error(codes.InvalidArgument, "Slug is required")
	}

	var userIDPtr *uuid.UUID
	if req.UserId != nil && *req.UserId != "" {
		id, err := uuid.Parse(*req.UserId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "Invalid user ID")
		}
		userIDPtr = &id
	}

	if err := s.reactionSvc.DeleteReaction(ctx, slug, userIDPtr); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to delete reaction: %v", err)
	}

	return &realmv1.DeleteReactionResponse{
		Status:  "success",
		Message: "Reaction deleted successfully",
	}, nil
}

