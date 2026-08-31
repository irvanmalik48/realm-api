package server

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/grpc/interceptors"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/service"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CommentServer struct {
	realmv1.UnimplementedCommentServiceServer
	commentSvc service.CommentService
}

func NewCommentServer(commentSvc service.CommentService) *CommentServer {
	return &CommentServer{commentSvc: commentSvc}
}

func mapCommentDTOToProto(dto *model.CommentDTO) *realmv1.Comment {
	if dto == nil {
		return nil
	}

	var parentIDStr *string
	if dto.ParentID != nil && *dto.ParentID != uuid.Nil {
		p := dto.ParentID.String()
		parentIDStr = &p
	}

	var replies []*realmv1.Comment
	for _, r := range dto.Replies {
		replies = append(replies, mapCommentDTOToProto(&r))
	}

	return &realmv1.Comment{
		Id:        dto.ID.String(),
		PostSlug:  dto.PostSlug,
		ParentId:  parentIDStr,
		Content:   dto.Content,
		IsEdited:  dto.IsEdited,
		IsAuthor:  dto.IsAuthor,
		CreatedAt: dto.CreatedAt.Format(time.RFC3339),
		UpdatedAt: dto.UpdatedAt.Format(time.RFC3339),
		Author: &realmv1.CommentAuthor{
			Id:        dto.Author.ID.String(),
			Username:  dto.Author.Username,
			FullName:  dto.Author.FullName,
			AvatarUrl: dto.Author.AvatarURL,
		},
		Replies: replies,
	}
}

func (s *CommentServer) GetComments(ctx context.Context, req *realmv1.GetCommentsRequest) (*realmv1.GetCommentsResponse, error) {
	slug := req.GetSlug()
	if slug == "" {
		return nil, status.Error(codes.InvalidArgument, "Slug is required")
	}

	var userIDPtr *uuid.UUID
	if id, ok := interceptors.GetUserID(ctx); ok {
		userIDPtr = &id
	}

	resp, err := s.commentSvc.GetComments(ctx, slug, userIDPtr)
	if err != nil {
		return nil, err
	}

	var comments []*realmv1.Comment
	for _, c := range resp.Comments {
		comments = append(comments, mapCommentDTOToProto(&c))
	}

	return &realmv1.GetCommentsResponse{
		Slug:       resp.Slug,
		TotalCount: int32(resp.TotalCount),
		Comments:   comments,
	}, nil
}

func (s *CommentServer) CreateComment(ctx context.Context, req *realmv1.CreateCommentRequest) (*realmv1.CommentResponse, error) {
	userID, err := interceptors.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}

	slug := req.GetSlug()
	if slug == "" {
		return nil, status.Error(codes.InvalidArgument, "Slug is required")
	}

	var parentID *uuid.UUID
	if req.ParentId != nil && *req.ParentId != "" {
		if pid, parseErr := uuid.Parse(*req.ParentId); parseErr == nil {
			parentID = &pid
		}
	}

	dto, err := s.commentSvc.CreateComment(ctx, slug, userID, req.GetContent(), parentID)
	if err != nil {
		return nil, err
	}

	return &realmv1.CommentResponse{
		Status:  "success",
		Comment: mapCommentDTOToProto(dto),
	}, nil
}

func (s *CommentServer) UpdateComment(ctx context.Context, req *realmv1.UpdateCommentRequest) (*realmv1.CommentResponse, error) {
	userID, err := interceptors.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}

	commentID, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid comment UUID")
	}

	dto, err := s.commentSvc.UpdateComment(ctx, commentID, userID, req.GetContent())
	if err != nil {
		return nil, err
	}

	return &realmv1.CommentResponse{
		Status:  "success",
		Comment: mapCommentDTOToProto(dto),
	}, nil
}

func (s *CommentServer) DeleteComment(ctx context.Context, req *realmv1.DeleteCommentRequest) (*realmv1.DeleteCommentResponse, error) {
	userID, err := interceptors.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}

	commentID, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid comment UUID")
	}

	if err := s.commentSvc.DeleteComment(ctx, commentID, userID); err != nil {
		return nil, err
	}

	return &realmv1.DeleteCommentResponse{
		Status:  "success",
		Message: "Comment deleted",
	}, nil
}
