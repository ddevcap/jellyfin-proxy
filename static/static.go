// Package static embeds static assets that are served by the proxy.
package static

import _ "embed"

// BrandingCSS is the custom CSS injected into the Jellyfin web UI via
// GET /Branding/Configuration and GET /Branding/Css to hide unsupported
// admin sections and server-side-only preference fields.
//
//go:embed branding.css
var BrandingCSS string
