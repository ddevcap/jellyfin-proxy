package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Session represents an access token issued by the proxy to a client.
// It mirrors the token Jellyfin issues on successful authentication so that
// compatible clients can use standard Jellyfin auth headers.
type Session struct {
	ent.Schema
}

func (Session) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		// The opaque token value sent by clients in X-Emby-Token / X-MediaBrowser-Token.
		field.String("token").
			Unique().
			NotEmpty().
			Sensitive(),
		// Jellyfin client identity fields — passed by clients during authentication.
		field.String("device_id").
			NotEmpty(),
		field.String("device_name").
			NotEmpty(),
		field.String("app_name").
			NotEmpty(),
		field.String("app_version").
			Optional(),
		field.Time("last_activity").
			Default(time.Now),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (Session) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("sessions").
			Unique().
			Required(),
	}
}

func (Session) Indexes() []ent.Index {
	return []ent.Index{
		// Fast token lookups on every authenticated request.
		index.Fields("token"),
	}
}
