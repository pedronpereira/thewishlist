package app

import (
	"strings"
	"testing"
)

func TestExtractOpenGraphImage(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		want    string
		wantErr bool
	}{
		{
			name: "og:image",
			html: `<html><head><meta property="og:image" content="https://shop.example/pic.jpg"></head></html>`,
			want: "https://shop.example/pic.jpg",
		},
		{
			name: "falls back to twitter:image when no og:image",
			html: `<html><head><meta name="twitter:image" content="https://shop.example/tw.jpg"></head></html>`,
			want: "https://shop.example/tw.jpg",
		},
		{
			name: "og:image wins even if it appears after twitter:image",
			html: `<html><head>
				<meta name="twitter:image" content="https://shop.example/tw.jpg">
				<meta property="og:image" content="https://shop.example/og.jpg">
			</head></html>`,
			want: "https://shop.example/og.jpg",
		},
		{
			name:    "no image meta tags at all",
			html:    `<html><head><title>Shop</title></head><body>no images here</body></html>`,
			wantErr: true,
		},
		{
			name:    "meta tag with empty content is ignored",
			html:    `<html><head><meta property="og:image" content=""></head></html>`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractOpenGraphImage(strings.NewReader(tt.html))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got image %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseFetchableURL_RejectsBadSchemesAndHosts(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"not a url", "::not a url::"},
		{"no scheme", "shop.example/product"},
		{"ftp scheme", "ftp://shop.example/product"},
		{"javascript scheme", "javascript:alert(1)"},
		{"loopback host", "http://127.0.0.1:8080/wishlist"},
		{"localhost", "http://localhost/wishlist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseFetchableURL(tt.url); err == nil {
				t.Fatalf("expected %q to be rejected, got no error", tt.url)
			}
		})
	}
}
