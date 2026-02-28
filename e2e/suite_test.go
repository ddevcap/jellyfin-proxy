//go:build e2e

// Package e2e contains end-to-end tests that run against a live Docker
// stack (proxy + Postgres + 2 Jellyfin backends).
//
// Run with: go test -tags e2e -v -count=1 -timeout 5m ./e2e/...
// Or:       make e2e
package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ── Configurable addresses ────────────────────────────────────────────────────

var (
	// proxyBase is the base URL of the running proxy.
	proxyBase = envOr("E2E_PROXY_URL", "http://localhost:18096")

	// jellyfinServer1 is the internal URL used by the proxy to reach server 1.
	// When registering backends, we use Docker service names since the proxy
	// runs inside Docker.
	jellyfinServer1 = envOr("E2E_JELLYFIN1_URL", "http://jellyfin-server1:8096")
	jellyfinServer2 = envOr("E2E_JELLYFIN2_URL", "http://jellyfin-server2:8096")

	// jellyfinServer1Direct is the URL reachable from the test runner (host).
	// Used to poll Jellyfin health before registering backends.
	// When both the proxy and Jellyfin are in Docker, we access them via
	// the proxy's port-mapping or dedicated published ports. Since we don't
	// publish Jellyfin ports in the e2e compose, we use the proxy itself
	// (which is healthy only when it can talk to Jellyfin) as the gate.
)

// ── Shared state populated by BeforeSuite ─────────────────────────────────────

var (
	adminToken string
	adminUser  userInfo
	userToken  string
	testUser   userInfo
	backend1ID string
	backend2ID string
)

type userInfo struct {
	ID       string
	Username string
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var _ = BeforeSuite(func() {
	By("Waiting for proxy to be healthy")
	waitForHealth(proxyBase+"/health", 120*time.Second)

	By("Logging in as admin")
	adminToken = login("admin", "e2e-admin-password")
	Expect(adminToken).NotTo(BeEmpty(), "admin login failed")

	By("Getting admin user info")
	adminUser = getCurrentUser(adminToken)

	By("Registering backend server 1")
	backend1ID = registerBackend("Server 1", jellyfinServer1, "s1")

	By("Registering backend server 2")
	backend2ID = registerBackend("Server 2", jellyfinServer2, "s2")

	By("Creating a test user")
	testUser = createProxyUser("e2euser", "e2e-test-password!")

	By("Mapping test user to backend server 1")
	loginToBackend(backend1ID, testUser.ID, "root", "password")

	By("Mapping test user to backend server 2")
	loginToBackend(backend2ID, testUser.ID, "root", "password")

	By("Logging in as test user")
	userToken = login("e2euser", "e2e-test-password!")
	Expect(userToken).NotTo(BeEmpty(), "test user login failed")

	By("Setup complete")
})

// ── Bootstrap helpers ─────────────────────────────────────────────────────────

func waitForHealth(url string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			return
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	Fail(fmt.Sprintf("proxy did not become healthy at %s within %s", url, timeout))
}

func login(username, password string) string {
	resp := post(proxyBase+"/users/authenticatebyname", map[string]string{
		"Username": username,
		"Pw":       password,
	}, "")
	defer resp.Body.Close()
	ExpectWithOffset(1, resp.StatusCode).To(Equal(http.StatusOK),
		fmt.Sprintf("login failed for %s: status %d", username, resp.StatusCode))

	var body map[string]interface{}
	Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
	return body["AccessToken"].(string)
}

func getCurrentUser(token string) userInfo {
	// The /users endpoint returns an array — the authenticated user is in it.
	resp := get(proxyBase+"/users", token)
	defer resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	var users []map[string]interface{}
	Expect(json.NewDecoder(resp.Body).Decode(&users)).To(Succeed())
	Expect(users).NotTo(BeEmpty())
	return userInfo{
		ID:       users[0]["Id"].(string),
		Username: users[0]["Name"].(string),
	}
}

func registerBackend(name, url, prefix string) string {
	resp := post(proxyBase+"/proxy/backends", map[string]interface{}{
		"name":   name,
		"url":    url,
		"prefix": prefix,
	}, adminToken)
	defer resp.Body.Close()
	ExpectWithOffset(1, resp.StatusCode).To(
		SatisfyAny(Equal(http.StatusCreated), Equal(http.StatusConflict)),
		fmt.Sprintf("register backend %s failed: status %d", name, resp.StatusCode))

	if resp.StatusCode == http.StatusConflict {
		// Backend already registered — find it.
		list := get(proxyBase+"/proxy/backends", adminToken)
		defer list.Body.Close()
		var backends []map[string]interface{}
		Expect(json.NewDecoder(list.Body).Decode(&backends)).To(Succeed())
		for _, b := range backends {
			if b["prefix"].(string) == prefix {
				return b["id"].(string)
			}
		}
		Fail(fmt.Sprintf("backend with prefix %s not found after conflict", prefix))
	}

	var body map[string]interface{}
	Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
	return body["id"].(string)
}

func createProxyUser(username, password string) userInfo {
	resp := post(proxyBase+"/proxy/users", map[string]interface{}{
		"username":     username,
		"display_name": username,
		"password":     password,
		"is_admin":     false,
	}, adminToken)
	defer resp.Body.Close()
	ExpectWithOffset(1, resp.StatusCode).To(
		SatisfyAny(Equal(http.StatusCreated), Equal(http.StatusConflict)),
		fmt.Sprintf("create user %s failed: status %d", username, resp.StatusCode))

	if resp.StatusCode == http.StatusConflict {
		// User already exists — find it.
		list := get(proxyBase+"/proxy/users", adminToken)
		defer list.Body.Close()
		var users []map[string]interface{}
		Expect(json.NewDecoder(list.Body).Decode(&users)).To(Succeed())
		for _, u := range users {
			if u["username"].(string) == username {
				return userInfo{ID: u["id"].(string), Username: username}
			}
		}
		Fail(fmt.Sprintf("user %s not found after conflict", username))
	}

	var body map[string]interface{}
	Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
	return userInfo{ID: body["id"].(string), Username: username}
}

func loginToBackend(backendID, proxyUserID, jfUser, jfPass string) {
	resp := post(proxyBase+"/proxy/backends/"+backendID+"/login", map[string]interface{}{
		"proxy_user_id": proxyUserID,
		"username":      jfUser,
		"password":      jfPass,
	}, adminToken)
	defer resp.Body.Close()
	ExpectWithOffset(1, resp.StatusCode).To(
		SatisfyAny(Equal(http.StatusCreated), Equal(http.StatusOK)),
		fmt.Sprintf("login to backend %s failed: status %d", backendID, resp.StatusCode))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

