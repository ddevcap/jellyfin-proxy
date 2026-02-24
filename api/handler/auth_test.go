package handler_test

import (
	"encoding/json"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gin-gonic/gin"

	"github.com/ddevcap/jellyfin-proxy/api/handler"
	"github.com/ddevcap/jellyfin-proxy/api/middleware"
	"github.com/ddevcap/jellyfin-proxy/config"
	"github.com/ddevcap/jellyfin-proxy/ent"
)

var _ = Describe("AuthHandler", func() {
	var router *gin.Engine

	testCfg := config.Config{
		ServerID:   "test-server-id",
		ServerName: "Test Proxy",
	}

	BeforeEach(func() {
		cleanDB()
		gin.SetMode(gin.TestMode)
		router = gin.New()
		h := handler.NewAuthHandler(db, testCfg, func(string) {}, func(string) {})
		router.POST("/Users/AuthenticateByName", h.AuthenticateByName)
		// Protected routes sit behind the Auth middleware so session validation
		// is exercised as part of the specs.
		auth := router.Group("/")
		auth.Use(middleware.Auth(db, testCfg))
		auth.POST("/Users/:userId/Password", h.UpdatePassword)
		auth.DELETE("/Sessions/Logout", h.Logout)
	})

	// ── AuthenticateByName ────────────────────────────────────────────────────

	Describe("AuthenticateByName", func() {
		Context("with valid credentials", func() {
			It("returns 200 with an access token and server info", func() {
				createUser("alice", "correctpass1", false)

				w := doPost(router, "/Users/AuthenticateByName", map[string]string{
					"Username": "alice",
					"Pw":       "correctpass1",
				})

				Expect(w.Code).To(Equal(http.StatusOK))
				var resp map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
				Expect(resp["AccessToken"]).NotTo(BeEmpty())
				Expect(resp["ServerId"]).To(Equal("test-server-id"))
			})
		})

		Context("with wrong password", func() {
			It("returns 401", func() {
				createUser("alice", "correctpass1", false)

				w := doPost(router, "/Users/AuthenticateByName", map[string]string{
					"Username": "alice",
					"Pw":       "wrongpass",
				})

				Expect(w.Code).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("with an unknown username", func() {
			It("returns 401", func() {
				w := doPost(router, "/Users/AuthenticateByName", map[string]string{
					"Username": "nobody",
					"Pw":       "whatever",
				})

				Expect(w.Code).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when the Username field is missing", func() {
			It("returns 400", func() {
				w := doPost(router, "/Users/AuthenticateByName", map[string]string{
					"Pw": "somepassword",
				})

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})
	})

	// ── UpdatePassword ────────────────────────────────────────────────────────

	Describe("UpdatePassword", func() {
		var user *ent.User

		BeforeEach(func() {
			user = createUser("bob", "oldpassword1", false)
			createSession(user, "bob-token")
		})

		Context("when the user changes their own password", func() {
			It("returns 204", func() {
				w := doPost(router, "/Users/"+user.ID.String()+"/Password",
					map[string]string{"CurrentPw": "oldpassword1", "NewPw": "newpassword1"},
					map[string]string{"X-Emby-Token": "bob-token"},
				)

				Expect(w.Code).To(Equal(http.StatusNoContent))
			})
		})

		Context("when the current password is wrong", func() {
			It("returns 403", func() {
				w := doPost(router, "/Users/"+user.ID.String()+"/Password",
					map[string]string{"CurrentPw": "wrongoldpass", "NewPw": "newpassword1"},
					map[string]string{"X-Emby-Token": "bob-token"},
				)

				Expect(w.Code).To(Equal(http.StatusForbidden))
			})
		})

		Context("when an admin resets another user's password", func() {
			It("returns 204 without requiring CurrentPw", func() {
				admin := createUser("admin", "adminpassword1", true)
				createSession(admin, "admin-token")

				w := doPost(router, "/Users/"+user.ID.String()+"/Password",
					map[string]interface{}{"NewPw": "freshpassword1"},
					map[string]string{"X-Emby-Token": "admin-token"},
				)

				Expect(w.Code).To(Equal(http.StatusNoContent))
			})
		})

		Context("when the new password is too short", func() {
			It("returns 400", func() {
				w := doPost(router, "/Users/"+user.ID.String()+"/Password",
					map[string]string{"CurrentPw": "oldpassword1", "NewPw": "short"},
					map[string]string{"X-Emby-Token": "bob-token"},
				)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Context("session invalidation on password change", func() {
			It("invalidates other sessions but keeps the caller's session", func() {
				// Create a second session for the same user.
				createSession(user, "bob-token-2")

				w := doPost(router, "/Users/"+user.ID.String()+"/Password",
					map[string]string{"CurrentPw": "oldpassword1", "NewPw": "newpassword1"},
					map[string]string{"X-Emby-Token": "bob-token"},
				)
				Expect(w.Code).To(Equal(http.StatusNoContent))

				// The caller's session (bob-token) should still work.
				w2 := doDelete(router, "/Sessions/Logout",
					map[string]string{"X-Emby-Token": "bob-token"},
				)
				Expect(w2.Code).To(Equal(http.StatusNoContent))

				// The other session (bob-token-2) should have been invalidated.
				w3 := doDelete(router, "/Sessions/Logout",
					map[string]string{"X-Emby-Token": "bob-token-2"},
				)
				Expect(w3.Code).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("without a valid session token", func() {
			It("returns 401", func() {
				w := doPost(router, "/Users/"+user.ID.String()+"/Password",
					map[string]string{"CurrentPw": "oldpassword1", "NewPw": "newpassword1"},
				)

				Expect(w.Code).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	// ── Logout ────────────────────────────────────────────────────────────────

	Describe("Logout", func() {
		It("returns 204 and invalidates the token so subsequent requests are rejected", func() {
			user := createUser("charlie", "password123", false)
			createSession(user, "charlie-token")

			w := doDelete(router, "/Sessions/Logout",
				map[string]string{"X-Emby-Token": "charlie-token"},
			)
			Expect(w.Code).To(Equal(http.StatusNoContent))

			// The same token is now gone — the auth middleware must reject it.
			w2 := doDelete(router, "/Sessions/Logout",
				map[string]string{"X-Emby-Token": "charlie-token"},
			)
			Expect(w2.Code).To(Equal(http.StatusUnauthorized))
		})
	})
})
