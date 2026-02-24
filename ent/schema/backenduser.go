package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// BackendUser maps a proxy User to their credentials on a specific Backend.
// A user can have at most one entry per backend.
type BackendUser struct {
	ent.Schema
}

func (BackendUser) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		// The user's ID on the backend Jellyfin server.
		field.String("backend_user_id").
			NotEmpty(),
		// Per-user auth token obtained from the backend server.
		// Optional: when absent, authenticated requests are sent without credentials.
		field.String("backend_token").
			Sensitive().
			Optional().
			Nillable(),
		field.Bool("enabled").
			Default(true),
	}
}

func (BackendUser) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("backend_users").
			Unique().
			Required(),
		edge.From("backend", Backend.Type).
			Ref("backend_users").
			Unique().
			Required(),
	}
}

func (BackendUser) Indexes() []ent.Index {
	return []ent.Index{
		// Enforce one mapping per (user, backend) pair.
		index.Edges("user", "backend").
			Unique(),
	}
}
