package store

import (
	"context"

	"github.com/AgentMesh-Net/indexer-go/internal/core/envelope"
)

// Cursor represents a pagination cursor for list queries.
type Cursor struct {
	CreatedAt string `json:"c"`
	ObjectID  string `json:"i"`
}

// Repo defines the storage interface for protocol objects.
type Repo interface {
	// InsertObject stores a validated envelope. Returns ErrConflict if object_id already exists.
	InsertObject(ctx context.Context, env *envelope.Envelope) error

	// ListObjects returns objects of the given type with cursor-based pagination.
	// Results are ordered by created_at DESC, object_id DESC.
	ListObjects(ctx context.Context, objectType string, limit int, cursor *Cursor) (items []envelope.Envelope, next *Cursor, err error)

	// GetObjectByID retrieves a single object by object_id.
	GetObjectByID(ctx context.Context, id string) (*envelope.Envelope, error)
}
