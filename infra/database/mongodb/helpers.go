package mongodb

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// MapError normalizes Mongo driver errors into domain AppErrors: missing
// documents become not_found, duplicate keys become conflict, everything else is
// an internal error preserving the cause.
func MapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return apperror.NotFound("resource not found")
	}
	if IsDuplicate(err) {
		return apperror.Conflict("resource already exists")
	}
	return apperror.Internal("database error").Wrap(err)
}

// IsDuplicate reports whether err is a Mongo duplicate-key (E11000) error.
func IsDuplicate(err error) bool {
	var we mongo.WriteException
	if errors.As(err, &we) {
		for _, e := range we.WriteErrors {
			if e.Code == 11000 {
				return true
			}
		}
	}
	var ce mongo.CommandError
	if errors.As(err, &ce) {
		return ce.Code == 11000
	}
	return false
}

// KeysetSort is the canonical sort for keyset pagination: newest first, with the
// id as a deterministic tiebreaker.
func KeysetSort() bson.D {
	return bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}
}

// ApplyKeyset returns a filter that combines base with the keyset cursor
// condition (created_at, _id) < cursor. An empty cursor returns base unchanged.
func ApplyKeyset(base bson.M, cur shared.Cursor) bson.M {
	if cur.ID == "" {
		return base
	}
	from := time.UnixMilli(cur.CreatedAt).UTC()
	keyset := bson.M{"$or": bson.A{
		bson.M{"created_at": bson.M{"$lt": from}},
		bson.M{"created_at": from, "_id": bson.M{"$lt": cur.ID}},
	}}
	return bson.M{"$and": bson.A{base, keyset}}
}

// CursorOf builds the opaque-cursor key from a document's timestamp and id.
func CursorOf(createdAt time.Time, id string) shared.Cursor {
	return shared.Cursor{CreatedAt: createdAt.UnixMilli(), ID: id}
}
