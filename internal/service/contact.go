package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/security"
)

type ContactService interface {
	SendMessage(ctx context.Context, req *model.ContactRequest, ipAddress, userAgent string) (*model.ContactSubmission, error)
}

type contactService struct {
	cfg        *config.Config
	repo       repository.ContactRepository
	httpClient *http.Client
}

func NewContactService(cfg *config.Config, repo repository.ContactRepository) ContactService {
	return &contactService{
		cfg:        cfg,
		repo:       repo,
		httpClient: security.NewSafeHTTPClient(10 * time.Second),
	}
}

func (s *contactService) SendMessage(ctx context.Context, req *model.ContactRequest, ipAddress, userAgent string) (*model.ContactSubmission, error) {
	log.Printf("[Contact] New message received from %s <%s> - Subject: %s\n", req.Name, req.Email, req.Subject)

	submission := &model.ContactSubmission{
		Name:      req.Name,
		Email:     req.Email,
		Subject:   req.Subject,
		Message:   req.Message,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	// Persist to PostgreSQL if repository is available
	if s.repo != nil {
		if err := s.repo.Create(ctx, submission); err != nil {
			return nil, fmt.Errorf("failed to save submission to database: %w", err)
		}
	}

	// Send notifications asynchronously or best-effort with SSRF protection
	if s.cfg.DiscordWebhookURL != "" {
		if err := security.ValidateDiscordWebhookURL(s.cfg.DiscordWebhookURL); err != nil {
			log.Printf("[Contact] SSRF guard blocked invalid Discord webhook: %v\n", err)
		} else {
			go func() {
				notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := s.sendDiscordNotification(notifyCtx, req); err != nil {
					log.Printf("[Contact] Failed to send Discord notification: %v\n", err)
				}
			}()
		}
	}

	if s.cfg.TelegramBotToken != "" && s.cfg.TelegramChatID != "" {
		if !security.ValidateTelegramBotToken(s.cfg.TelegramBotToken) {
			log.Println("[Contact] SSRF guard blocked invalid Telegram bot token format")
		} else {
			go func() {
				notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := s.sendTelegramNotification(notifyCtx, req); err != nil {
					log.Printf("[Contact] Failed to send Telegram notification: %v\n", err)
				}
			}()
		}
	}

	return submission, nil
}

func (s *contactService) sendDiscordNotification(ctx context.Context, req *model.ContactRequest) error {
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title": fmt.Sprintf("📬 New Contact Message: %s", req.Subject),
				"color": 3447003, // Blue
				"fields": []map[string]interface{}{
					{"name": "From", "value": req.Name, "inline": true},
					{"name": "Email", "value": req.Email, "inline": true},
					{"name": "Message", "value": req.Message, "inline": false},
				},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.DiscordWebhookURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("discord returned status %d", resp.StatusCode)
	}

	return nil
}

func (s *contactService) sendTelegramNotification(ctx context.Context, req *model.ContactRequest) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.cfg.TelegramBotToken)

	msgText := fmt.Sprintf("📬 *New Contact Message*\n\n*Name:* %s\n*Email:* %s\n*Subject:* %s\n\n*Message:*\n%s",
		req.Name, req.Email, req.Subject, req.Message)

	data := url.Values{}
	data.Set("chat_id", s.cfg.TelegramChatID)
	data.Set("text", msgText)
	data.Set("parse_mode", "Markdown")

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram returned status %d", resp.StatusCode)
	}

	return nil
}
