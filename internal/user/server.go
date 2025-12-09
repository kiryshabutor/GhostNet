package user

import (
	"context"

	userv1 "ghostnet/gen/go/proto/user/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service реализует gRPC контракты user-service.
type Service struct {
	userv1.UnimplementedUserServiceServer
	repo   *Repository
	logger *zap.Logger
}

func NewService(repo *Repository, logger *zap.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

func (s *Service) GetOrCreateUser(ctx context.Context, req *userv1.GetOrCreateUserRequest) (*userv1.GetOrCreateUserResponse, error) {
	if req.GetTelegramId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "telegram_id is required")
	}

	userID, err := s.repo.GetOrCreateUser(ctx, req.GetTelegramId())
	if err != nil {
		s.logger.Error("failed to get or create user", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to store user")
	}

	return &userv1.GetOrCreateUserResponse{UserId: userID}, nil
}

func (s *Service) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	if req.GetUserId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	telegramID, err := s.repo.GetUser(ctx, req.GetUserId())
	if err != nil {
		if err == ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		s.logger.Error("failed to get user", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	return &userv1.GetUserResponse{
		UserId:     req.GetUserId(),
		TelegramId: telegramID,
	}, nil
}

func (s *Service) GetNotificationSettings(ctx context.Context, req *userv1.GetNotificationSettingsRequest) (*userv1.GetNotificationSettingsResponse, error) {
	if req.GetUserId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	settings, err := s.repo.GetNotificationSettings(ctx, req.GetUserId())
	if err != nil {
		if err == ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		s.logger.Error("failed to get notification settings", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get settings")
	}

	return &userv1.GetNotificationSettingsResponse{
		NotifyOnLike:    settings.NotifyOnLike,
		NotifyOnDislike: settings.NotifyOnDislike,
		NotifyOnComment: settings.NotifyOnComment,
	}, nil
}

func (s *Service) UpdateNotificationSettings(ctx context.Context, req *userv1.UpdateNotificationSettingsRequest) (*userv1.UpdateNotificationSettingsResponse, error) {
	if req.GetUserId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	settings := NotificationSettings{
		NotifyOnLike:    req.GetNotifyOnLike(),
		NotifyOnDislike: req.GetNotifyOnDislike(),
		NotifyOnComment: req.GetNotifyOnComment(),
	}

	if err := s.repo.UpsertNotificationSettings(ctx, req.GetUserId(), settings); err != nil {
		if err == ErrUserNotFound {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		s.logger.Error("failed to update notification settings", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to update settings")
	}

	return &userv1.UpdateNotificationSettingsResponse{}, nil
}
