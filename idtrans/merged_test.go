package idtrans_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ddevcap/jellyfin-proxy/idtrans"
)

var _ = Describe("EncodeMerged", func() {
	DescribeTable("prefixes the content type with 'merged_'",
		func(contentType, want string) {
			Expect(idtrans.EncodeMerged(contentType)).To(Equal(want))
		},
		Entry("movies", "movies", "merged_movies"),
		Entry("tvshows", "tvshows", "merged_tvshows"),
		Entry("music", "music", "merged_music"),
	)
})

var _ = Describe("DecodeMerged", func() {
	DescribeTable("recognises valid merged IDs",
		func(id, wantContentType string) {
			ct, ok := idtrans.DecodeMerged(id)
			Expect(ok).To(BeTrue())
			Expect(ct).To(Equal(wantContentType))
		},
		Entry("movies", "merged_movies", "movies"),
		Entry("tvshows", "merged_tvshows", "tvshows"),
		Entry("music", "merged_music", "music"),
	)

	DescribeTable("rejects non-merged IDs",
		func(id string) {
			_, ok := idtrans.DecodeMerged(id)
			Expect(ok).To(BeFalse())
		},
		Entry("regular proxy ID", "s1_abc123"),
		Entry("bare ID without prefix", "abc123"),
		Entry("merged prefix without underscore", "mergedmovies"),
		Entry("empty string", ""),
		Entry("proxy-prefixed merged ID", "s1_merged_movies"),
	)

	It("round-trips with EncodeMerged for various content types", func() {
		for _, ct := range []string{"movies", "tvshows", "music", "books", "boxsets"} {
			encoded := idtrans.EncodeMerged(ct)
			decoded, ok := idtrans.DecodeMerged(encoded)
			Expect(ok).To(BeTrue(), "DecodeMerged(%q) returned ok=false", encoded)
			Expect(decoded).To(Equal(ct))
		}
	})
})
