package handler

import (
	"net/http"
	"time"

	"github.com/ddevcap/jellyfin-proxy/ent"
	entbackenduser "github.com/ddevcap/jellyfin-proxy/ent/backenduser"
	entuser "github.com/ddevcap/jellyfin-proxy/ent/user"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ProxyUserHandler manages proxy-local user accounts via the admin REST API.
type ProxyUserHandler struct {
	db *ent.Client
}

func NewProxyUserHandler(db *ent.Client) *ProxyUserHandler {
	return &ProxyUserHandler{db: db}
}

// userResponse is the outward representation of a proxy user.
// hashed_password is intentionally omitted.
type userResponse struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	IsAdmin     bool      `json:"is_admin"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toUserResponse(u *ent.User) userResponse {
	return userResponse{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		IsAdmin:     u.IsAdmin,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

// ── Create ────────────────────────────────────────────────────────────────────

type createUserRequest struct {
	Username    string `json:"username"     binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	Password    string `json:"password"     binding:"required,min=8"`
	IsAdmin     bool   `json:"is_admin"`
}

// CreateUser handles POST /proxy/users.
func (h *ProxyUserHandler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), BcryptCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user, err := h.db.User.Create().
		SetUsername(req.Username).
		SetDisplayName(req.DisplayName).
		SetHashedPassword(string(hash)).
		SetIsAdmin(req.IsAdmin).
		Save(c.Request.Context())
	if err != nil {
		if ent.IsConstraintError(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, toUserResponse(user))
}

// ── List ──────────────────────────────────────────────────────────────────────

// ListUsers handles GET /proxy/users.
func (h *ProxyUserHandler) ListUsers(c *gin.Context) {
	users, err := h.db.User.Query().
		Order(entuser.ByUsername()).
		All(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}

	resp := make([]userResponse, len(users))
	for i, u := range users {
		resp[i] = toUserResponse(u)
	}
	c.JSON(http.StatusOK, resp)
}

// ── Get ───────────────────────────────────────────────────────────────────────

// GetProxyUser handles GET /proxy/users/:id.
func (h *ProxyUserHandler) GetProxyUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	user, err := h.db.User.Get(c.Request.Context(), id)
	if err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}

// ── Update ────────────────────────────────────────────────────────────────────

// updateUserRequest uses pointer fields so that absent fields are distinguished
// from zero-values, enabling true partial updates.
type updateUserRequest struct {
	DisplayName *string `json:"display_name"`
	Password    *string `json:"password"`
	IsAdmin     *bool   `json:"is_admin"`
}

// UpdateUser handles PATCH /proxy/users/:id.
func (h *ProxyUserHandler) UpdateUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	upd := h.db.User.UpdateOneID(id)
	changed := false

	if req.DisplayName != nil {
		if *req.DisplayName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "display_name cannot be empty"})
			return
		}
		upd.SetDisplayName(*req.DisplayName)
		changed = true
	}

	if req.Password != nil {
		if len(*req.Password) < 8 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), BcryptCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
			return
		}
		upd.SetHashedPassword(string(hash))
		changed = true
	}

	if req.IsAdmin != nil {
		upd.SetIsAdmin(*req.IsAdmin)
		changed = true
	}

	if !changed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields provided to update"})
		return
	}

	user, err := upd.Save(c.Request.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}

// ── User backends ─────────────────────────────────────────────────────────────

// userBackendResponse is the user-centric view of a single BackendUser mapping.
type userBackendResponse struct {
	MappingID     uuid.UUID `json:"mapping_id"`
	BackendID     uuid.UUID `json:"backend_id"`
	BackendName   string    `json:"backend_name"`
	BackendURL    string    `json:"backend_url"`
	Prefix        string    `json:"prefix"`
	BackendUserID string    `json:"backend_user_id"`
	Enabled       bool      `json:"enabled"`
}

// GetUserBackends handles GET /proxy/users/:id/backends.
// Returns all backend mappings for the given proxy user, with backend details
// inlined so callers don't need a second request per backend.
func (h *ProxyUserHandler) GetUserBackends(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	if _, err := h.db.User.Get(c.Request.Context(), id); err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	mappings, err := h.db.BackendUser.Query().
		Where(entbackenduser.HasUserWith(entuser.ID(id))).
		WithBackend().
		All(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list backend mappings"})
		return
	}

	resp := make([]userBackendResponse, len(mappings))
	for i, m := range mappings {
		r := userBackendResponse{
			MappingID:     m.ID,
			BackendUserID: m.BackendUserID,
			Enabled:       m.Enabled,
		}
		if m.Edges.Backend != nil {
			r.BackendID = m.Edges.Backend.ID
			r.BackendName = m.Edges.Backend.Name
			r.BackendURL = m.Edges.Backend.URL
			r.Prefix = m.Edges.Backend.Prefix
		}
		resp[i] = r
	}
	c.JSON(http.StatusOK, resp)
}

// ── Delete ────────────────────────────────────────────────────────────────────

// DeleteUser handles DELETE /proxy/users/:id.
func (h *ProxyUserHandler) DeleteUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	// Prevent admins from deleting their own account.
	if caller := userFromCtx(c); caller != nil && caller.ID == id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete your own account"})
		return
	}

	err = h.db.User.DeleteOneID(id).Exec(c.Request.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user"})
		return
	}

	c.Status(http.StatusNoContent)
}
