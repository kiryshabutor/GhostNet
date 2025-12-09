package interaction

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	eventsv1 "ghostnet/gen/go/proto/events/v1"
	interactionv1 "ghostnet/gen/go/proto/interaction/v1"
	"ghostnet/internal/common/kafka"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	topicPostEvents = "post-events"
)

// Service реализует InteractionService.
type Service struct {
	interactionv1.UnimplementedInteractionServiceServer
	repo     *Repository
	producer *kafka.Producer
	logger   *zap.Logger
}

func NewService(repo *Repository, producer *kafka.Producer, logger *zap.Logger) *Service {
	return &Service{
		repo:     repo,
		producer: producer,
		logger:   logger,
	}
}

func (s *Service) LikePost(ctx context.Context, req *interactionv1.LikePostRequest) (*interactionv1.LikePostResponse, error) {
	if req.GetUserId() == 0 || req.GetPostId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id and post_id are required")
	}

	authorID, err := s.repo.GetPostAuthor(ctx, req.GetPostId())
	if err != nil {
		if err == ErrPostNotFound {
			return nil, status.Error(codes.NotFound, "post not found")
		}
		s.logger.Error("failed to get post author", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to process like")
	}

	if err := s.repo.UpsertVote(ctx, req.GetUserId(), req.GetPostId(), 1); err != nil {
		s.logger.Error("failed to upsert like", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to like post")
	}

	s.publishEvent(ctx, eventsv1.EventType_EVENT_TYPE_POST_LIKED, req.GetPostId(), req.GetUserId(), authorID, 0)

	return &interactionv1.LikePostResponse{}, nil
}

func (s *Service) DislikePost(ctx context.Context, req *interactionv1.DislikePostRequest) (*interactionv1.DislikePostResponse, error) {
	if req.GetUserId() == 0 || req.GetPostId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id and post_id are required")
	}

	authorID, err := s.repo.GetPostAuthor(ctx, req.GetPostId())
	if err != nil {
		if err == ErrPostNotFound {
			return nil, status.Error(codes.NotFound, "post not found")
		}
		s.logger.Error("failed to get post author", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to process dislike")
	}

	if err := s.repo.UpsertVote(ctx, req.GetUserId(), req.GetPostId(), -1); err != nil {
		s.logger.Error("failed to upsert dislike", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to dislike post")
	}

	s.publishEvent(ctx, eventsv1.EventType_EVENT_TYPE_POST_DISLIKED, req.GetPostId(), req.GetUserId(), authorID, 0)

	return &interactionv1.DislikePostResponse{}, nil
}

func (s *Service) AddComment(ctx context.Context, req *interactionv1.AddCommentRequest) (*interactionv1.AddCommentResponse, error) {
	if req.GetUserId() == 0 || req.GetPostId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id and post_id are required")
	}
	if strings.TrimSpace(req.GetText()) == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}

	authorID, err := s.repo.GetPostAuthor(ctx, req.GetPostId())
	if err != nil {
		if err == ErrPostNotFound {
			return nil, status.Error(codes.NotFound, "post not found")
		}
		s.logger.Error("failed to get post author", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to add comment")
	}

	commentID, err := s.repo.AddComment(ctx, req.GetUserId(), req.GetPostId(), req.GetText())
	if err != nil {
		s.logger.Error("failed to add comment", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to add comment")
	}

	s.publishEvent(ctx, eventsv1.EventType_EVENT_TYPE_COMMENT_ADDED, req.GetPostId(), req.GetUserId(), authorID, commentID)

	return &interactionv1.AddCommentResponse{CommentId: commentID}, nil
}

func (s *Service) GetPostStats(ctx context.Context, req *interactionv1.GetPostStatsRequest) (*interactionv1.GetPostStatsResponse, error) {
	if req.GetPostId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "post_id is required")
	}

	if _, err := s.repo.GetPostAuthor(ctx, req.GetPostId()); err != nil {
		if err == ErrPostNotFound {
			return nil, status.Error(codes.NotFound, "post not found")
		}
		s.logger.Error("failed to validate post", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get stats")
	}

	likes, dislikes, comments, views, err := s.repo.GetStats(ctx, req.GetPostId())
	if err != nil {
		s.logger.Error("failed to get stats", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get stats")
	}

	return &interactionv1.GetPostStatsResponse{
		Likes:    likes,
		Dislikes: dislikes,
		Comments: comments,
		Views:    views,
	}, nil
}

func (s *Service) ListPostComments(ctx context.Context, req *interactionv1.ListPostCommentsRequest) (*interactionv1.ListPostCommentsResponse, error) {
	if req.GetPostId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "post_id is required")
	}
	if req.GetPageSize() <= 0 {
		req.PageSize = 10
	}

	if _, err := s.repo.GetPostAuthor(ctx, req.GetPostId()); err != nil {
		if err == ErrPostNotFound {
			return nil, status.Error(codes.NotFound, "post not found")
		}
		s.logger.Error("failed to validate post", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to list comments")
	}

	items, hasMore, err := s.repo.ListComments(ctx, req.GetPostId(), req.GetPage(), req.GetPageSize())
	if err != nil {
		s.logger.Error("failed to list comments", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to list comments")
	}

	resp := &interactionv1.ListPostCommentsResponse{
		Comments: make([]*interactionv1.CommentItem, 0, len(items)),
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
		HasMore:  hasMore,
	}

	for _, item := range items {
		resp.Comments = append(resp.Comments, &interactionv1.CommentItem{
			CommentId: item.ID,
			Text:      item.Text,
			CreatedAt: item.CreatedAt,
		})
	}

	return resp, nil
}

func (s *Service) MarkPostViewed(ctx context.Context, req *interactionv1.MarkPostViewedRequest) (*interactionv1.MarkPostViewedResponse, error) {
	if req.GetUserId() == 0 || req.GetPostId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id and post_id are required")
	}

	authorID, err := s.repo.GetPostAuthor(ctx, req.GetPostId())
	if err != nil {
		if err == ErrPostNotFound {
			return nil, status.Error(codes.NotFound, "post not found")
		}
		s.logger.Error("failed to validate post", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to mark view")
	}

	created, err := s.repo.MarkViewed(ctx, req.GetUserId(), req.GetPostId())
	if err != nil {
		s.logger.Error("failed to mark viewed", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to mark viewed")
	}

	if created {
		s.publishEvent(ctx, eventsv1.EventType_EVENT_TYPE_POST_VIEWED, req.GetPostId(), req.GetUserId(), authorID, 0)
	}

	return &interactionv1.MarkPostViewedResponse{}, nil
}

func (s *Service) publishEvent(ctx context.Context, eventType eventsv1.EventType, postID, actorID, postAuthorID, commentID int64) {
	if s.producer == nil {
		return
	}

	event := &eventsv1.PostEvent{
		EventId:      fmt.Sprintf("evt-%d-%d", postID, time.Now().UnixNano()),
		EventType:    eventType,
		PostId:       postID,
		ActorUserId:  actorID,
		PostAuthorId: postAuthorID,
		CommentId:    commentID,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}

	payload, err := proto.Marshal(event)
	if err != nil {
		s.logger.Error("failed to marshal event", zap.Error(err))
		return
	}

	key := []byte(strconv.FormatInt(postID, 10))
	if err := s.producer.Send(ctx, topicPostEvents, key, payload); err != nil {
		s.logger.Error("failed to publish interaction event", zap.Error(err))
	}
}
