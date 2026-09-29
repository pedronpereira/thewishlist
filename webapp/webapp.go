package webapp

import (
	"crypto/subtle"
	"embed"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pedronpereira/thewishlist/internal/app"
)

//go:embed views/*.html
var viewsFS embed.FS

type templateRenderer struct {
	templates *template.Template
}

func (t *templateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// New builds a fully-wired Echo instance (routes, renderer, static assets)
// without starting a server, so both cmd/main.go and the Vercel entrypoint
// can share the exact same setup.
func New() (*echo.Echo, error) {
	e := echo.New()
	e.Use(middleware.Logger())
	useBasicAuth(e)
	useCSRFProtection(e)
	e.Static("/css", "public/css")

	tmpl, err := template.ParseFS(viewsFS, "views/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	e.Renderer = &templateRenderer{templates: tmpl}

	a := app.New()
	if err := a.Init(); err != nil {
		return nil, fmt.Errorf("initializing app: %w", err)
	}
	a.RegisterHandlers(e)

	return e, nil
}

// useBasicAuth protects every route with a single shared username/password,
// read from WISHLIST_USERNAME/WISHLIST_PASSWORD. If WISHLIST_PASSWORD isn't
// set, the site is left unprotected — with a startup log making that
// visible, so an unset password on a real deployment isn't a silent gap.
func useBasicAuth(e *echo.Echo) {
	password := os.Getenv("WISHLIST_PASSWORD")
	if password == "" {
		fmt.Println("WISHLIST_PASSWORD not set: the wishlist is NOT password protected")
		return
	}

	username := os.Getenv("WISHLIST_USERNAME")
	if username == "" {
		username = "family"
	}

	e.Use(middleware.BasicAuth(func(u, p string, c echo.Context) (bool, error) {
		validUser := subtle.ConstantTimeCompare([]byte(u), []byte(username)) == 1
		validPass := subtle.ConstantTimeCompare([]byte(p), []byte(password)) == 1
		return validUser && validPass, nil
	}))
}

// useCSRFProtection guards the one browser-driven mutation in this app: the
// "Comprei" (buy) button, rendered on GET / and submitted via an HTMX POST.
// It's scoped to just those two routes rather than applied globally, so the
// JSON API endpoints (used directly via curl/Bruno for administration) don't
// need a token dance. Uses Echo's default double-submit-cookie CSRF
// middleware; the token is read back from the X-CSRF-Token header, which the
// template sets via an inherited hx-headers attribute.
func useCSRFProtection(e *echo.Echo) {
	e.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
		Skipper: func(c echo.Context) bool {
			switch {
			case c.Request().Method == http.MethodGet && c.Path() == "/":
				return false
			case c.Request().Method == http.MethodPost && c.Path() == "/wishitem/:id/buy":
				return false
			default:
				return true
			}
		},
	}))
}
