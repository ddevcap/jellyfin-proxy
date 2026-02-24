package idtrans_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ddevcap/jellyfin-proxy/idtrans"
)

type obj = map[string]interface{}

// rewriteResponse is a test helper that marshals input, runs RewriteResponse,
// and returns the unmarshalled result. Expectations are inline so Ginkgo
// reports the correct spec location on failure.
func rewriteResponse(input obj, prefix, proxyServerID string) obj { //nolint:unparam
	b, err := json.Marshal(input)
	Expect(err).NotTo(HaveOccurred())
	result, err := idtrans.RewriteResponse(b, prefix, proxyServerID)
	Expect(err).NotTo(HaveOccurred())
	var out obj
	Expect(json.Unmarshal(result, &out)).To(Succeed())
	return out
}

// rewriteRequest is a test helper that marshals input, runs RewriteRequest,
// and returns the unmarshalled result.
func rewriteRequest(input obj) obj {
	b, err := json.Marshal(input)
	Expect(err).NotTo(HaveOccurred())
	result, err := idtrans.RewriteRequest(b)
	Expect(err).NotTo(HaveOccurred())
	var out obj
	Expect(json.Unmarshal(result, &out)).To(Succeed())
	return out
}

// ── Encode ────────────────────────────────────────────────────────────────────

var _ = Describe("Encode", func() {
	It("prefixes the backendID with prefix and underscore", func() {
		Expect(idtrans.Encode("s1", "abc123")).To(Equal("s1_abc123"))
	})

	It("returns an empty string when backendID is empty", func() {
		Expect(idtrans.Encode("s1", "")).To(BeEmpty())
	})
})

// ── Decode ────────────────────────────────────────────────────────────────────

var _ = Describe("Decode", func() {
	DescribeTable("correctly splits prefix and backendID",
		func(proxyID, wantPrefix, wantBackendID string) {
			prefix, backendID, err := idtrans.Decode(proxyID)
			Expect(err).NotTo(HaveOccurred())
			Expect(prefix).To(Equal(wantPrefix))
			Expect(backendID).To(Equal(wantBackendID))
		},
		Entry("simple alphanumeric ID", "s1_abc123", "s1", "abc123"),
		Entry("UUID with hyphens as backendID", "s2_a1b2c3d4-e5f6-7890-abcd-ef1234567890", "s2", "a1b2c3d4-e5f6-7890-abcd-ef1234567890"),
		Entry("backendID that itself contains underscores", "s1_has_underscore", "s1", "has_underscore"),
	)

	Context("when the ID has no underscore separator", func() {
		It("returns an error", func() {
			_, _, err := idtrans.Decode("noprefixhere")
			Expect(err).To(HaveOccurred())
		})

		It("returns the original value as backendID so callers can pass it through", func() {
			_, backendID, _ := idtrans.Decode("noprefixhere")
			Expect(backendID).To(Equal("noprefixhere"))
		})
	})

	It("round-trips with Encode", func() {
		prefix, backendID := "s1", "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
		encoded := idtrans.Encode(prefix, backendID)
		gotPrefix, gotBackend, err := idtrans.Decode(encoded)
		Expect(err).NotTo(HaveOccurred())
		Expect(gotPrefix).To(Equal(prefix))
		Expect(gotBackend).To(Equal(backendID))
	})
})

// ── RewriteResponse ───────────────────────────────────────────────────────────

var _ = Describe("RewriteResponse", func() {
	Context("top-level ID fields", func() {
		It("prefixes Id and ParentId, replaces ServerId, leaves non-ID fields unchanged", func() {
			out := rewriteResponse(obj{
				"Id":       "abc123",
				"ParentId": "def456",
				"ServerId": "backend-server-uuid",
				"Name":     "My Movie",
			}, "s1", "proxy-server-id")

			Expect(out["Id"]).To(Equal("s1_abc123"))
			Expect(out["ParentId"]).To(Equal("s1_def456"))
			Expect(out["ServerId"]).To(Equal("proxy-server-id"))
			Expect(out["Name"]).To(Equal("My Movie"))
		})
	})

	Context("nested Items array", func() {
		It("rewrites Id and ServerId in each item", func() {
			out := rewriteResponse(obj{
				"Id": "parent",
				"Items": []interface{}{
					obj{"Id": "child1", "ServerId": "backend-id", "Name": "Episode 1"},
					obj{"Id": "child2", "ServerId": "backend-id", "Name": "Episode 2"},
				},
			}, "s1", "proxy-id")

			Expect(out["Id"]).To(Equal("s1_parent"))
			items := out["Items"].([]interface{})
			item0 := items[0].(obj)
			item1 := items[1].(obj)
			Expect(item0["Id"]).To(Equal("s1_child1"))
			Expect(item0["ServerId"]).To(Equal("proxy-id"))
			Expect(item1["Id"]).To(Equal("s1_child2"))
			Expect(item1["ServerId"]).To(Equal("proxy-id"))
		})
	})

	Context("UserData sub-object", func() {
		It("rewrites ItemId inside UserData", func() {
			out := rewriteResponse(obj{
				"Id": "abc123",
				"UserData": obj{
					"ItemId":     "abc123",
					"Played":     false,
					"IsFavorite": false,
				},
			}, "s1", "proxy-id")

			Expect(out["Id"]).To(Equal("s1_abc123"))
			userData := out["UserData"].(obj)
			Expect(userData["ItemId"]).To(Equal("s1_abc123"))
		})
	})

	Context("ArtistItems array", func() {
		It("rewrites Id inside each ArtistItem", func() {
			out := rewriteResponse(obj{
				"Id": "album1",
				"ArtistItems": []interface{}{
					obj{"Id": "artist1", "Name": "Artist One"},
				},
			}, "s1", "proxy-id")

			Expect(out["Id"]).To(Equal("s1_album1"))
			artists := out["ArtistItems"].([]interface{})
			Expect(artists[0].(obj)["Id"]).To(Equal("s1_artist1"))
		})
	})

	Context("empty string IDs", func() {
		It("leaves empty ID fields unchanged", func() {
			out := rewriteResponse(obj{"Id": "abc", "ParentId": ""}, "s1", "proxy-id")

			Expect(out["Id"]).To(Equal("s1_abc"))
			Expect(out["ParentId"]).To(Equal(""))
		})
	})

	Context("null IDs", func() {
		It("passes null fields through unchanged", func() {
			raw := []byte(`{"Id":"abc","SeriesId":null}`)
			result, err := idtrans.RewriteResponse(raw, "s1", "proxy-id")
			Expect(err).NotTo(HaveOccurred())
			var out obj
			Expect(json.Unmarshal(result, &out)).To(Succeed())
			Expect(out["Id"]).To(Equal("s1_abc"))
			Expect(out["SeriesId"]).To(BeNil())
		})
	})
})

// ── RewriteRequest ────────────────────────────────────────────────────────────

var _ = Describe("RewriteRequest", func() {
	Context("proxied IDs (with prefix)", func() {
		It("strips the prefix, leaving non-ID fields untouched", func() {
			out := rewriteRequest(obj{
				"ItemId":    "s1_abc123",
				"SomeOther": "untouched",
			})

			Expect(out["ItemId"]).To(Equal("abc123"))
			Expect(out["SomeOther"]).To(Equal("untouched"))
		})

		It("leaves ServerId unchanged", func() {
			out := rewriteRequest(obj{"Id": "s1_abc", "ServerId": "proxy-server-id"})

			Expect(out["Id"]).To(Equal("abc"))
			Expect(out["ServerId"]).To(Equal("proxy-server-id"))
		})
	})

	Context("non-proxied IDs (without prefix)", func() {
		It("passes the ID through unchanged", func() {
			out := rewriteRequest(obj{"Id": "noprefixhere"})

			Expect(out["Id"]).To(Equal("noprefixhere"))
		})
	})
})
