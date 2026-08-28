package model

import (
	"time"

	"github.com/google/uuid"
)

// AllowedReactionTypes defines the set of valid reaction identifiers
var AllowedReactionTypes = map[string]bool{
	"like":      true,
	"love":      true,
	"fire":      true,
	"rocket":    true,
	"mindblown": true,
	"party":     true,
}

// PostReaction represents the database row for a post reaction
type PostReaction struct {
	ID               uuid.UUID  `json:"id"`
	PostSlug         string     `json:"post_slug"`
	ReactionType     string     `json:"reaction_type"`
	UserID           *uuid.UUID `json:"user_id,omitempty"`
	ClientIdentifier string     `json:"client_identifier"`
	CreatedAt        time.Time  `json:"created_at"`
}

// PostReactionsResponse represents the summary of reactions for a blog post
type PostReactionsResponse struct {
	Slug          string         `json:"slug"`
	TotalCount    int            `json:"total_count"`
	Reactions     map[string]int `json:"reactions"`
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
	UserReactions []string       `json:"user_reactions"`
}
