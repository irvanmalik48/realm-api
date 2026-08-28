package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
)

type ReactionRepository interface {
	GetReactionsBySlug(ctx context.Context, slug string, clientIdentifier string) (*model.PostReactionsResponse, error)
	ToggleReaction(ctx context.Context, slug string, reactionType string, userID *uuid.UUID, clientIdentifier string) (*model.ToggleReactionResponse, error)
}

type reactionRepository struct {
	db *database.DB
}

func NewReactionRepository(db *database.DB) ReactionRepository {
	return &reactionRepository{db: db}
}

func (r *reactionRepository) GetReactionsBySlug(ctx context.Context, slug string, clientIdentifier string) (*model.PostReactionsResponse, error) {
	reactionsMap := make(map[string]int)
	for k := range model.AllowedReactionTypes {
		reactionsMap[k] = 0
	}

	totalCount := 0

	// 1. Get counts grouped by reaction_type
	countQuery := `
		SELECT reaction_type, COUNT(*) 
		FROM post_reactions 
		WHERE post_slug = $1 
		GROUP BY reaction_type
	`
	rows, err := r.db.Pool.Query(ctx, countQuery, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to query reaction counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rType string
		var count int
		if err := rows.Scan(&rType, &count); err == nil {
			reactionsMap[rType] = count
			totalCount += count
		}
	}

	// 2. Get user's active reactions
	userReactions := make([]string, 0)
	if clientIdentifier != "" {
		userQuery := `
			SELECT reaction_type 
			FROM post_reactions 
			WHERE post_slug = $1 AND client_identifier = $2
		`
		uRows, err := r.db.Pool.Query(ctx, userQuery, slug, clientIdentifier)
		if err == nil {
			defer uRows.Close()
			for uRows.Next() {
				var rType string
				if err := uRows.Scan(&rType); err == nil {
					userReactions = append(userReactions, rType)
				}
			}
		}
	}

	return &model.PostReactionsResponse{
		Slug:          slug,
		TotalCount:    totalCount,
		Reactions:     reactionsMap,
		UserReactions: userReactions,
	}, nil
}

func (r *reactionRepository) ToggleReaction(ctx context.Context, slug string, reactionType string, userID *uuid.UUID, clientIdentifier string) (*model.ToggleReactionResponse, error) {
	// Check if already reacted
	var existingID uuid.UUID
	checkQuery := `
		SELECT id FROM post_reactions 
		WHERE post_slug = $1 AND reaction_type = $2 AND client_identifier = $3
	`
	err := r.db.Pool.QueryRow(ctx, checkQuery, slug, reactionType, clientIdentifier).Scan(&existingID)

	active := false
	if err == nil {
		// Existing reaction found -> remove it (toggle off)
		delQuery := `DELETE FROM post_reactions WHERE id = $1`
		if _, err := r.db.Pool.Exec(ctx, delQuery, existingID); err != nil {
			return nil, fmt.Errorf("failed to delete reaction: %w", err)
		}
		active = false
	} else {
		// No existing reaction -> insert it (toggle on)
		insQuery := `
			INSERT INTO post_reactions (post_slug, reaction_type, user_id, client_identifier)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (post_slug, reaction_type, client_identifier) DO NOTHING
		`
		if _, err := r.db.Pool.Exec(ctx, insQuery, slug, reactionType, userID, clientIdentifier); err != nil {
			return nil, fmt.Errorf("failed to insert reaction: %w", err)
		}
		active = true
	}

	// Fetch updated summary
	summary, err := r.GetReactionsBySlug(ctx, slug, clientIdentifier)
	if err != nil {
		return nil, err
	}

	return &model.ToggleReactionResponse{
		Slug:          slug,
		Reaction:      reactionType,
		Active:        active,
		TotalCount:    summary.TotalCount,
		Reactions:     summary.Reactions,
		UserReactions: summary.UserReactions,
	}, nil
}
