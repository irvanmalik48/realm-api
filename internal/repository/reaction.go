package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
)

type ReactionRepository interface {
	GetReactionsBySlug(ctx context.Context, slug string, userID *uuid.UUID) (*model.PostReactionsResponse, error)
	ToggleReaction(ctx context.Context, slug string, reactionType string, userID uuid.UUID) (*model.ToggleReactionResponse, error)
}

type reactionRepository struct {
	db *database.DB
}

func NewReactionRepository(db *database.DB) ReactionRepository {
	return &reactionRepository{db: db}
}

func (r *reactionRepository) GetReactionsBySlug(ctx context.Context, slug string, userID *uuid.UUID) (*model.PostReactionsResponse, error) {
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

	// 2. Get user's active reaction (at most 1 per post)
	var userReaction *string
	userReactions := make([]string, 0)
	if userID != nil && *userID != uuid.Nil {
		userQuery := `
			SELECT reaction_type 
			FROM post_reactions 
			WHERE post_slug = $1 AND user_id = $2
			LIMIT 1
		`
		var rType string
		if err := r.db.Pool.QueryRow(ctx, userQuery, slug, *userID).Scan(&rType); err == nil {
			userReaction = &rType
			userReactions = append(userReactions, rType)
		}
	}

	return &model.PostReactionsResponse{
		Slug:          slug,
		TotalCount:    totalCount,
		Reactions:     reactionsMap,
		UserReaction:  userReaction,
		UserReactions: userReactions,
	}, nil
}

func (r *reactionRepository) ToggleReaction(ctx context.Context, slug string, reactionType string, userID uuid.UUID) (*model.ToggleReactionResponse, error) {
	// Check existing reaction for this user on this post
	var existingID uuid.UUID
	var existingReaction string
	checkQuery := `
		SELECT id, reaction_type FROM post_reactions 
		WHERE post_slug = $1 AND user_id = $2
	`
	err := r.db.Pool.QueryRow(ctx, checkQuery, slug, userID).Scan(&existingID, &existingReaction)

	active := false
	if err == nil {
		if existingReaction == reactionType {
			// User clicked the same active reaction -> remove it (toggle off)
			delQuery := `DELETE FROM post_reactions WHERE id = $1`
			if _, err := r.db.Pool.Exec(ctx, delQuery, existingID); err != nil {
				return nil, fmt.Errorf("failed to delete reaction: %w", err)
			}
			active = false
		} else {
			// User clicked a different reaction -> update to new reaction (switch reaction)
			updQuery := `UPDATE post_reactions SET reaction_type = $1, created_at = NOW() WHERE id = $2`
			if _, err := r.db.Pool.Exec(ctx, updQuery, reactionType, existingID); err != nil {
				return nil, fmt.Errorf("failed to update reaction: %w", err)
			}
			active = true
		}
	} else {
		// No existing reaction -> insert new reaction
		insQuery := `
			INSERT INTO post_reactions (post_slug, reaction_type, user_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (post_slug, user_id) DO UPDATE SET reaction_type = EXCLUDED.reaction_type, created_at = NOW()
		`
		if _, err := r.db.Pool.Exec(ctx, insQuery, slug, reactionType, userID); err != nil {
			return nil, fmt.Errorf("failed to insert reaction: %w", err)
		}
		active = true
	}

	// Fetch updated summary
	summary, err := r.GetReactionsBySlug(ctx, slug, &userID)
	if err != nil {
		return nil, err
	}

	return &model.ToggleReactionResponse{
		Slug:          slug,
		Reaction:      reactionType,
		Active:        active,
		TotalCount:    summary.TotalCount,
		Reactions:     summary.Reactions,
		UserReaction:  summary.UserReaction,
		UserReactions: summary.UserReactions,
	}, nil
}
