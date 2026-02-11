package store

import "time"

// ObjectRow represents a row in the objects table.
type ObjectRow struct {
	ObjectID      string
	ObjectType    string
	ObjectVersion string
	CreatedAt     time.Time
	SignerPubKey  string
	EnvelopeJSON  []byte
	PayloadJSON   []byte
	InsertedAt    time.Time
}
