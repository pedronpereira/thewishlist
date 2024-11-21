package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Wishlist struct {
	Count     int
	Tshirts   []WishItem
	Books     []WishItem
	MouseMats []WishItem
}

type WishItem struct {
	Id           string
	Name         string
	Title        string
	Description  string
	ItemType     string
	ShopUrl      string
	WasPurchased bool
	ImgSource    string
}

type Templates struct {
	templates *template.Template
}

func (t *Templates) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func newTemplate() *Templates {
	return &Templates{
		templates: template.Must(template.ParseGlob("views/*.html")),
	}
}

func main() {
	e := echo.New()
	e.Use(middleware.Logger())

	e.Static("/css", "css")
	e.Renderer = newTemplate()

	//load the file
	data, err := os.ReadFile("./resources/wishlist.json")
	if err != nil {
		fmt.Printf("ERROR %s: %v", "Reading file", err)
	}

	var payload Wishlist
	err = json.Unmarshal(data, &payload)
	if err != nil {
		fmt.Printf("ERROR %s: %v", "Parsing json", err)
	}
	e.GET("/wishlist", func(c echo.Context) error {
		// return c.String(http.StatusOK, "wishlist will be here!")
		return c.JSON(http.StatusOK, payload)
	})

	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "index", payload)
	})
	e.Logger.Fatal(e.Start(":43067"))

	//TODO:
}
