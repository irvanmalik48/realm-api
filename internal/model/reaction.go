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
	ID           uuid.UUID `json:"id"`
	PostSlug     string    `json:"post_slug"`
	ReactionType string    `json:"reaction_type"`
	UserID       uuid.UUID `json:"user_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// PostReactionsResponse represents the summary of reactions for a blog post
type PostReactionsResponse struct {
	Slug          string         `json:"slug"`
	TotalCount    int            `json:"total_count"`
	Reactions     map[string]int `json:"reactions"`
	UserReaction  *string        `json:"user_reaction"`
	UserReactions []string       `json:"user_reactions"`
}

// ToggleReactionRequest is the payload sent by client to toggle a reaction
type ToggleReactionRequest struct {
	Reaction string `json:"reaction"`
}

// ToggleReactionResponse is the response returned after toggling a reaction
type ToggleReactionResponse struct {
	Slug          string         `json:"slug"`
	Reaction      string         `json:"reaction"`
	Active        bool           `json:"active"`
	TotalCount    int            `json:"total_count"`
	Reactions     map[string]int `json:"reactions"`
	UserReaction  *string        `json:"user_reaction"`
	UserReactions []string       `json:"user_reactions"`
}
