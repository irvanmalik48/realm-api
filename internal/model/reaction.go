package model

import (
	"time"

	"github.com/google/uuid"
)

// AllowedReactionTypes defines the set of valid reaction identifiers (3 positive, 3 negative)
var AllowedReactionTypes = map[string]bool{
	"like":    true,
	"love":    true,
	"fire":    true,
	"dislike": true,
	"frown":   true,
	"skull":   true,
}

// PostReaction represents the database row for a post reaction
type PostReaction struct {
	CreatedAt    time.Time `json:"created_at"`
	PostSlug     string    `json:"post_slug"`
	ReactionType string    `json:"reaction_type"`
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
}

// PostReactionsResponse represents the summary of reactions for a blog post
type PostReactionsResponse struct {
	Reactions     map[string]int `json:"reactions"`
	UserReaction  *string        `json:"user_reaction"`
	Slug          string         `json:"slug"`
	UserReactions []string       `json:"user_reactions"`
	TotalCount    int            `json:"total_count"`
}

// ToggleReactionRequest is the payload sent by client to toggle a reaction
type ToggleReactionRequest struct {
	Reaction string `json:"reaction"`
}

// ToggleReactionResponse is the response returned after toggling a reaction
type ToggleReactionResponse struct {
	Reactions     map[string]int `json:"reactions"`
	UserReaction  *string        `json:"user_reaction"`
	Slug          string         `json:"slug"`
	Reaction      string         `json:"reaction"`
	UserReactions []string       `json:"user_reactions"`
	TotalCount    int            `json:"total_count"`
	Active        bool           `json:"active"`
}

type ReactionSummaryDTO struct {
	Reactions  map[string]int `json:"reactions"`
	Slug       string         `json:"slug"`
	TotalCount int            `json:"total_count"`
}
