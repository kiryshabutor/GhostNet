package feed

import (
	"context"

	feedv1 "ghostnet/gen/go/proto/feed/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service реализует FeedService.
type Service struct {
	feedv1.UnimplementedFeedServiceServer
	repo   *Repository
	logger *zap.Logger
	stop   context.CancelFunc
}

func NewService(repo *Repository, logger *zap.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

func (s *Service) GetNextPostForUser(ctx context.Context, req *feedv1.GetNextPostForUserRequest) (*feedv1.GetNextPostForUserResponse, error) {
	if req.GetUserId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	postID, authorID, found, err := s.repo.NextPost(ctx, req.GetUserId())
	if err != nil {
		s.logger.Error("failed to get next post", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get next post")
	}

	if !found {
		return &feedv1.GetNextPostForUserResponse{HasPost: false}, nil
	}

	return &feedv1.GetNextPostForUserResponse{
		HasPost:      true,
		PostId:       postID,
		AuthorUserId: authorID,
	}, nil
}
