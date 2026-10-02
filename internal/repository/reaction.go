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
	GetSummaries(ctx context.Context, limit, offset int, search string) ([]model.ReactionSummaryDTO, error)
	DeleteReaction(ctx context.Context, slug string, userID *uuid.UUID) error
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
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check existing reaction for this user on this post with row lock
	var existingID uuid.UUID
	var existingReaction string
	checkQuery := `
		SELECT id, reaction_type FROM post_reactions 
		WHERE post_slug = $1 AND user_id = $2
		FOR UPDATE
	`
	err = tx.QueryRow(ctx, checkQuery, slug, userID).Scan(&existingID, &existingReaction)

	active := false
	if err == nil {
		if existingReaction == reactionType {
			// User clicked the same active reaction -> remove it (toggle off)
			delQuery := `DELETE FROM post_reactions WHERE id = $1`
			if _, err := tx.Exec(ctx, delQuery, existingID); err != nil {
				return nil, fmt.Errorf("failed to delete reaction: %w", err)
			}
			active = false
		} else {
			// User clicked a different reaction -> update to new reaction (switch reaction)
			updQuery := `UPDATE post_reactions SET reaction_type = $1, created_at = NOW() WHERE id = $2`
			if _, err := tx.Exec(ctx, updQuery, reactionType, existingID); err != nil {
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
		if _, err := tx.Exec(ctx, insQuery, slug, reactionType, userID); err != nil {
			return nil, fmt.Errorf("failed to insert reaction: %w", err)
		}
		active = true
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit reaction transaction: %w", err)
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

func (r *reactionRepository) GetSummaries(ctx context.Context, limit, offset int, search string) ([]model.ReactionSummaryDTO, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	whereClause := ""
	args := []interface{}{}
	if search != "" {
		whereClause = "WHERE post_slug ILIKE $1"
		args = append(args, "%"+search+"%")
	}

	query := fmt.Sprintf(`
		SELECT post_slug, reaction_type, COUNT(*)
		FROM post_reactions
		%s
		GROUP BY post_slug, reaction_type
		ORDER BY post_slug ASC
	`, whereClause)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query reactions summary: %w", err)
	}
	defer rows.Close()

	summaryMap := make(map[string]map[string]int)
	orderedSlugs := make([]string, 0)

	for rows.Next() {
		var slug, rType string
		var count int
		if err := rows.Scan(&slug, &rType, &count); err == nil {
			if _, exists := summaryMap[slug]; !exists {
				summaryMap[slug] = make(map[string]int)
				for k := range model.AllowedReactionTypes {
					summaryMap[slug][k] = 0
				}
				orderedSlugs = append(orderedSlugs, slug)
			}
			summaryMap[slug][rType] = count
		}
	}

	// Apply pagination in memory over unique slugs
	start := offset
	if start > len(orderedSlugs) {
		start = len(orderedSlugs)
	}
	end := start + limit
	if end > len(orderedSlugs) {
		end = len(orderedSlugs)
	}
	pagedSlugs := orderedSlugs[start:end]

	results := make([]model.ReactionSummaryDTO, 0, len(pagedSlugs))
	for _, slug := range pagedSlugs {
		counts := summaryMap[slug]
		total := 0
		for _, c := range counts {
			total += c
		}
		results = append(results, model.ReactionSummaryDTO{
			Slug:       slug,
			TotalCount: total,
			Reactions:  counts,
		})
	}

	return results, nil
}

func (r *reactionRepository) DeleteReaction(ctx context.Context, slug string, userID *uuid.UUID) error {
	var query string
	var args []interface{}

	if userID != nil && *userID != uuid.Nil {
		query = "DELETE FROM post_reactions WHERE post_slug = $1 AND user_id = $2"
		args = []interface{}{slug, *userID}
	} else {
		query = "DELETE FROM post_reactions WHERE post_slug = $1"
		args = []interface{}{slug}
	}

	_, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete reaction: %w", err)
	}
	return nil
}

