package app

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	imageFetchTimeout   = 8 * time.Second
	imageFetchMaxBytes  = 3 << 20 // 3 MiB is plenty for a page's <head>
	imageFetchUserAgent = "Mozilla/5.0 (compatible; TheWishlistBot/1.0; +https://github.com/pedronpereira/thewishlist)"
)

var errNoImageFound = errors.New("não foi possível encontrar uma imagem nessa página")

// fetchOpenGraphImage retrieves the page at rawURL and returns the first
// og:image meta tag it finds (falling back to twitter:image), resolved to
// an absolute URL. It backs the add-item form's "Buscar imagem" button,
// which pre-fills the image field from a pasted shop link instead of
// asking the admin to go find and copy an image URL by hand.
func fetchOpenGraphImage(rawURL string) (string, error) {
	pageURL, err := parseFetchableURL(rawURL)
	if err != nil {
		return "", err
	}

	client := &http.Client{
		Timeout: imageFetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("demasiados redireccionamentos")
			}
			_, err := parseFetchableURL(req.URL.String())
			return err
		},
	}

	req, err := http.NewRequest(http.MethodGet, pageURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("link inválido: %w", err)
	}
	req.Header.Set("User-Agent", imageFetchUserAgent)
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("não foi possível aceder a esse link: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("a loja respondeu com o estado %d", resp.StatusCode)
	}

	imageURL, err := extractOpenGraphImage(io.LimitReader(resp.Body, imageFetchMaxBytes))
	if err != nil {
		return "", err
	}

	resolved, err := resp.Request.URL.Parse(imageURL)
	if err != nil {
		return "", fmt.Errorf("a imagem encontrada tem um link inválido: %w", err)
	}

	return resolved.String(), nil
}

// parseFetchableURL validates that rawURL is an absolute http(s) URL whose
// host doesn't resolve to a loopback, private, or link-local address (e.g.
// cloud metadata endpoints), so the admin-only fetch-image endpoint can't
// be used to probe internal network services.
func parseFetchableURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("link inválido: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("o link tem de ser http ou https")
	}
	host := parsed.Hostname()
	if host == "" {
		return nil, errors.New("link inválido")
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("não foi possível resolver esse link: %w", err)
	}
	if slices.ContainsFunc(ips, isDisallowedImageFetchIP) {
		return nil, errors.New("esse link não é permitido")
	}

	return parsed, nil
}

func isDisallowedImageFetchIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// extractOpenGraphImage scans r for the first og:image meta tag, falling
// back to twitter:image if no og:image is present anywhere in the page.
func extractOpenGraphImage(r io.Reader) (string, error) {
	tokenizer := html.NewTokenizer(r)
	var fallback string

	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			if fallback != "" {
				return fallback, nil
			}
			return "", errNoImageFound
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data != "meta" {
				continue
			}
			property, content := metaProperty(token)
			if content == "" {
				continue
			}
			switch property {
			case "og:image", "og:image:url", "og:image:secure_url":
				return content, nil
			case "twitter:image", "twitter:image:src":
				if fallback == "" {
					fallback = content
				}
			}
		}
	}
}

func metaProperty(token html.Token) (property, content string) {
	for _, attr := range token.Attr {
		switch strings.ToLower(attr.Key) {
		case "property", "name":
			if property == "" {
				property = strings.ToLower(attr.Val)
			}
		case "content":
			content = attr.Val
		}
	}
	return property, content
}
