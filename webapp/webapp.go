package webapp

import (
	"embed"
	"fmt"
	"html/template"
	"io"

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
