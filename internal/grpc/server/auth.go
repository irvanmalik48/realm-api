package server

import (
	"context"
	"time"

	"github.com/irvanmalik48/realm-api/internal/grpc/interceptors"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/service"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
)

type AuthServer struct {
	realmv1.UnimplementedAuthServiceServer
	authSvc service.AuthService
}

func NewAuthServer(authSvc service.AuthService) *AuthServer {
	return &AuthServer{authSvc: authSvc}
}

func mapUserDTOToProto(dto *model.UserDTO) *realmv1.User {
	if dto == nil {
		return nil
	}

	var accounts []*realmv1.OAuthAccount
	for _, a := range dto.ConnectedAccounts {
		accounts = append(accounts, &realmv1.OAuthAccount{
			Provider:  a.Provider,
			Email:     a.Email,
			AvatarUrl: a.AvatarURL,
			CreatedAt: dto.CreatedAt.Format(time.RFC3339),
		})
	}

	return &realmv1.User{
		Id:                dto.ID.String(),
		Email:             dto.Email,
		Username:          dto.Username,
		FullName:          dto.FullName,
		AvatarUrl:         dto.AvatarURL,
		Provider:          dto.Provider,
		CreatedAt:         dto.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         dto.CreatedAt.Format(time.RFC3339),
		ConnectedAccounts: accounts,
	}
}

func (s *AuthServer) Register(ctx context.Context, req *realmv1.RegisterRequest) (*realmv1.AuthResponse, error) {
	input := model.RegisterInput{
		Email:     req.GetEmail(),
		Username:  req.GetUsername(),
		Password:  req.GetPassword(),
		FullName:  req.GetFullName(),
		AvatarURL: req.AvatarUrl,
	}

	resp, err := s.authSvc.Register(ctx, input)
	if err != nil {
		return nil, err
	}

	return &realmv1.AuthResponse{
		Status:  resp.Status,
		Message: resp.Message,
		Token:   resp.Token,
		User:    mapUserDTOToProto(resp.User),
	}, nil
}

func (s *AuthServer) Login(ctx context.Context, req *realmv1.LoginRequest) (*realmv1.AuthResponse, error) {
	input := model.LoginInput{
		Identifier: req.GetIdentifier(),
		Password:   req.GetPassword(),
	}

	resp, err := s.authSvc.Login(ctx, input)
	if err != nil {
		return nil, err
	}

	return &realmv1.AuthResponse{
		Status:  resp.Status,
		Message: resp.Message,
		Token:   resp.Token,
		User:    mapUserDTOToProto(resp.User),
	}, nil
}

func (s *AuthServer) GetProfile(ctx context.Context, req *realmv1.GetProfileRequest) (*realmv1.ProfileResponse, error) {
	userID, err := interceptors.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}

	dto, err := s.authSvc.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &realmv1.ProfileResponse{
		Status: "success",
		User:   mapUserDTOToProto(dto),
	}, nil
}

func (s *AuthServer) UpdateProfile(ctx context.Context, req *realmv1.UpdateProfileRequest) (*realmv1.ProfileResponse, error) {
	userID, err := interceptors.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}

	input := model.UpdateProfileInput{
		FullName:  req.FullName,
		Username:  req.Username,
		AvatarURL: req.AvatarUrl,
	}

	dto, err := s.authSvc.UpdateProfile(ctx, userID, input)
	if err != nil {
		return nil, err
	}

	msg := "Profile updated successfully"
	return &realmv1.ProfileResponse{
		Status:  "success",
		Message: &msg,
		User:    mapUserDTOToProto(dto),
	}, nil
}

func (s *AuthServer) SetPassword(ctx context.Context, req *realmv1.SetPasswordRequest) (*realmv1.SetPasswordResponse, error) {
	userID, err := interceptors.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}

	input := model.SetPasswordInput{
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.GetNewPassword(),
	}

	if err := s.authSvc.SetPassword(ctx, userID, input); err != nil {
		return nil, err
	}

	return &realmv1.SetPasswordResponse{
		Status:  "success",
		Message: "Password updated successfully",
	}, nil
}

func (s *AuthServer) CheckAvailability(ctx context.Context, req *realmv1.CheckAvailabilityRequest) (*realmv1.CheckAvailabilityResponse, error) {
	resp, err := s.authSvc.CheckAvailability(ctx, req.GetUsername(), req.GetEmail())
	if err != nil {
		return nil, err
	}

	var res realmv1.CheckAvailabilityResponse
	if resp.UsernameAvailable != nil {
		res.UsernameAvailable = resp.UsernameAvailable
		if resp.UsernameReason != "" {
			res.UsernameReason = &resp.UsernameReason
		}
	}
	if resp.EmailAvailable != nil {
		res.EmailAvailable = resp.EmailAvailable
		if resp.EmailReason != "" {
			res.EmailReason = &resp.EmailReason
		}
	}

	return &res, nil
}

func (s *AuthServer) UnlinkOAuth(ctx context.Context, req *realmv1.UnlinkOAuthRequest) (*realmv1.UnlinkOAuthResponse, error) {
	userID, err := interceptors.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.authSvc.UnlinkOAuthAccount(ctx, userID, req.GetProvider()); err != nil {
		return nil, err
	}

	return &realmv1.UnlinkOAuthResponse{
		Status:  "success",
		Message: "Account unlinked successfully",
	}, nil
}
