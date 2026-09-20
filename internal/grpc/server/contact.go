package server

import (
	"context"
	"log"
	"regexp"
	"strings"

	"sync"
	"time"

	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/service"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type contactIPLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ContactServer struct {
	realmv1.UnimplementedContactServiceServer
	contactSvc service.ContactService
	mu         sync.Mutex
	limiters   map[string]*contactIPLimiter
}

func NewContactServer(contactSvc service.ContactService) *ContactServer {
	return &ContactServer{
		contactSvc: contactSvc,
		limiters:   make(map[string]*contactIPLimiter),
	}
}

func (s *ContactServer) isAllowed(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	// Periodic eviction if map grows
	if len(s.limiters) > 1000 {
		for k, v := range s.limiters {
			if now.Sub(v.lastSeen) > 30*time.Minute {
				delete(s.limiters, k)
			}
		}
	}

	entry, exists := s.limiters[ip]
	if !exists {
		// 5 requests per 10 minutes = 1 token per 2 minutes, burst 5
		lim := rate.NewLimiter(rate.Every(2*time.Minute), 5)
		entry = &contactIPLimiter{limiter: lim, lastSeen: now}
		s.limiters[ip] = entry
	}
	entry.lastSeen = now

	return entry.limiter.Allow()
}

func (s *ContactServer) SendMessage(ctx context.Context, req *realmv1.SendMessageRequest) (*realmv1.SendMessageResponse, error) {
	name := strings.TrimSpace(req.GetName())
	email := strings.TrimSpace(req.GetEmail())
	subject := strings.TrimSpace(req.GetSubject())
	message := strings.TrimSpace(req.GetMessage())
	gotcha := strings.TrimSpace(req.GetGotcha())

	// Honeypot detection
	if gotcha != "" {
		log.Println("[Contact gRPC] Honeypot triggered, dropping submission silently.")
		return &realmv1.SendMessageResponse{
			Status:  "success",
			Message: "Your message has been sent successfully.",
		}, nil
	}

	if name == "" || len(name) < 2 || len(name) > 100 {
		return nil, status.Error(codes.InvalidArgument, "Name must be between 2 and 100 characters")
	}

	if email == "" || !emailRegex.MatchString(email) || len(email) > 254 {
		return nil, status.Error(codes.InvalidArgument, "Invalid email address")
	}

	if subject == "" || len(subject) < 3 || len(subject) > 200 {
		return nil, status.Error(codes.InvalidArgument, "Subject must be between 3 and 200 characters")
	}

	if message == "" || len(message) < 10 || len(message) > 5000 {
		return nil, status.Error(codes.InvalidArgument, "Message must be between 10 and 5000 characters")
	}

	submission := &model.ContactRequest{
		Name:     name,
		Email:    email,
		Subject:  subject,
		Message:  message,
		Honeypot: gotcha,
	}

	ipAddress := req.GetIpAddress()
	if ipAddress == "" {
		ipAddress = "127.0.0.1"
	}

	if !s.isAllowed(ipAddress) {
		return nil, status.Error(codes.ResourceExhausted, "Too many contact requests. Please try again later.")
	}

	userAgent := req.GetUserAgent()

	if _, err := s.contactSvc.SendMessage(ctx, submission, ipAddress, userAgent); err != nil {
		log.Printf("[Contact gRPC] Failed to save message: %v\n", err)
		return nil, status.Error(codes.Internal, "Failed to save message")
	}

	return &realmv1.SendMessageResponse{
		Status:  "success",
		Message: "Your message has been sent successfully.",
	}, nil
}
