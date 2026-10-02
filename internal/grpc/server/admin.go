package server

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AdminServer struct {
	realmv1.UnimplementedAdminRBACServiceServer
	adminRepo repository.AdminRepository
}

func NewAdminServer(adminRepo repository.AdminRepository) *AdminServer {
	return &AdminServer{
		adminRepo: adminRepo,
	}
}

func mapAdminUserToProto(admin *model.AdminUser) *realmv1.AdminUser {
	if admin == nil {
		return nil
	}

	return &realmv1.AdminUser{
		Id:           admin.ID.String(),
		IsSuperadmin: admin.IsSuperadmin,
		Permissions:  admin.Permissions,
		CreatedAt:    admin.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    admin.UpdatedAt.Format(time.RFC3339),
		User: &realmv1.User{
			Id:        admin.User.ID.String(),
			Email:     admin.User.Email,
			Username:  admin.User.Username,
			FullName:  admin.User.FullName,
			AvatarUrl: admin.User.AvatarURL,
			Provider:  admin.User.Provider,
			CreatedAt: admin.User.CreatedAt.Format(time.RFC3339),
		},
	}
}

func (s *AdminServer) ListAdmins(ctx context.Context, req *realmv1.ListAdminsRequest) (*realmv1.ListAdminsResponse, error) {
	admins, err := s.adminRepo.ListAdmins(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to list admins: %v", err)
	}

	protoAdmins := make([]*realmv1.AdminUser, len(admins))
	for i := range admins {
		protoAdmins[i] = mapAdminUserToProto(&admins[i])
	}

	return &realmv1.ListAdminsResponse{
		Admins: protoAdmins,
	}, nil
}

func (s *AdminServer) AddAdmin(ctx context.Context, req *realmv1.AddAdminRequest) (*realmv1.AdminUser, error) {
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "Email cannot be empty")
	}

	admin, err := s.adminRepo.AddAdmin(ctx, req.GetEmail(), req.GetPermissions(), nil)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Failed to add admin: %v", err)
	}

	return mapAdminUserToProto(admin), nil
}

func (s *AdminServer) UpdateAdminPermissions(ctx context.Context, req *realmv1.UpdateAdminPermissionsRequest) (*realmv1.AdminUser, error) {
	adminID, err := uuid.Parse(req.GetAdminId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid admin UUID")
	}

	admin, err := s.adminRepo.UpdatePermissions(ctx, adminID, req.GetPermissions())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Failed to update permissions: %v", err)
	}

	return mapAdminUserToProto(admin), nil
}

func (s *AdminServer) RemoveAdmin(ctx context.Context, req *realmv1.RemoveAdminRequest) (*realmv1.AdminOperationResponse, error) {
	adminID, err := uuid.Parse(req.GetAdminId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid admin UUID")
	}

	if err := s.adminRepo.RemoveAdmin(ctx, adminID); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Failed to remove admin: %v", err)
	}

	return &realmv1.AdminOperationResponse{
		Status:  "success",
		Message: "Admin removed successfully",
	}, nil
}
