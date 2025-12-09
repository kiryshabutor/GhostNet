package post

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	eventsv1 "ghostnet/gen/go/proto/events/v1"
	postv1 "ghostnet/gen/go/proto/post/v1"
	"ghostnet/internal/common/kafka"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	topicPostEvents = "post-events"
)

// Service реализует gRPC PostService.
type Service struct {
	postv1.UnimplementedPostServiceServer
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

func (s *Service) CreatePost(ctx context.Context, req *postv1.CreatePostRequest) (*postv1.CreatePostResponse, error) {
	if req.GetAuthorUserId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "author_user_id is required")
	}
	text := strings.TrimSpace(req.GetText())
	if text == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}

	mediaType := strings.TrimSpace(req.GetMediaType())
	if mediaType != "" && mediaType != "photo" && mediaType != "video" {
		return nil, status.Error(codes.InvalidArgument, "media_type must be empty, photo or video")
	}
	if mediaType != "" && strings.TrimSpace(req.GetTelegramFileId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "telegram_file_id is required when media_type set")
	}

	postID, err := s.repo.CreatePost(ctx, req.GetAuthorUserId(), text, mediaType, req.GetTelegramFileId(), req.GetTelegramUniqueId())
	if err != nil {
		s.logger.Error("failed to create post", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to create post")
	}

	s.publishPostCreated(ctx, postID, req.GetAuthorUserId())

	return &postv1.CreatePostResponse{PostId: postID}, nil
}

func (s *Service) GetPost(ctx context.Context, req *postv1.GetPostRequest) (*postv1.GetPostResponse, error) {
	if req.GetPostId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "post_id is required")
	}

	post, err := s.repo.GetPost(ctx, req.GetPostId())
	if err != nil {
		if err == ErrPostNotFound {
			return nil, status.Error(codes.NotFound, "post not found")
		}
		s.logger.Error("failed to get post", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get post")
	}

	return &postv1.GetPostResponse{
		PostId:           post.ID,
		AuthorUserId:     post.AuthorUserID,
		Text:             post.Text,
		MediaType:        post.MediaType,
		TelegramFileId:   post.TelegramFileID,
		TelegramUniqueId: post.TelegramUniqueID,
	}, nil
}

func (s *Service) ListUserPosts(ctx context.Context, req *postv1.ListUserPostsRequest) (*postv1.ListUserPostsResponse, error) {
	if req.GetUserId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.GetPageSize() <= 0 {
		req.PageSize = 10
	}

	items, hasMore, err := s.repo.ListUserPosts(ctx, req.GetUserId(), req.GetPage(), req.GetPageSize())
	if err != nil {
		s.logger.Error("failed to list posts", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to list posts")
	}

	resp := &postv1.ListUserPostsResponse{
		Posts:    make([]*postv1.PostItem, 0, len(items)),
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
		HasMore:  hasMore,
	}

	for _, item := range items {
		resp.Posts = append(resp.Posts, &postv1.PostItem{
			PostId:      item.PostID,
			TextPreview: item.TextPreview,
			Likes:       item.Likes,
			Dislikes:    item.Dislikes,
			Comments:    item.Comments,
			Views:       item.Views,
		})
	}

	return resp, nil
}

func (s *Service) publishPostCreated(ctx context.Context, postID int64, authorID int64) {
	if s.producer == nil {
		return
	}

	event := &eventsv1.PostEvent{
		EventId:      fmt.Sprintf("post-%d-%d", postID, time.Now().UnixNano()),
		EventType:    eventsv1.EventType_EVENT_TYPE_POST_CREATED,
		PostId:       postID,
		ActorUserId:  authorID,
		PostAuthorId: authorID,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}

	payload, err := proto.Marshal(event)
	if err != nil {
		s.logger.Error("failed to marshal post created event", zap.Error(err))
		return
	}

	key := []byte(strconv.FormatInt(postID, 10))
	if err := s.producer.Send(ctx, topicPostEvents, key, payload); err != nil {
		s.logger.Error("failed to publish post created event", zap.Error(err))
	}
}
