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
	Count   int
	Tshirts []WishItem
	Books   []WishItem
	Other   []WishItem
}

// Searches all items for the purchased item and returns the updated item
func (w *Wishlist) ItemPurchased(id string) *WishItem {
	for i, wishitem := range w.Other {
		if wishitem.Id == id {
			wishitem.WasPurchased = !wishitem.WasPurchased
			w.Other[i] = wishitem
			return &wishitem
		}
	}

	for i, wishitem := range w.Books {
		if wishitem.Id == id {
			wishitem.WasPurchased = !wishitem.WasPurchased
			w.Books[i] = wishitem
			return &wishitem
		}
	}

	for i, wishitem := range w.Tshirts {
		if wishitem.Id == id {
			wishitem.WasPurchased = !wishitem.WasPurchased
			w.Tshirts[i] = wishitem
			return &wishitem
		}
	}

	return nil
}

func (w *Wishlist) UpdateItem(requestItem WishItem) (string, error) {
	if requestItem.Id == "" {
		return "", fmt.Errorf("item has no id")
	}

	if requestItem.ItemType == "" {
		return "", fmt.Errorf("item has no type")
	}

	if requestItem.ItemType == "t-shirt" {
		for i, wishitem := range w.Tshirts {
			if wishitem.Id == requestItem.Id {
				w.Tshirts[i] = requestItem
				return requestItem.ItemType, nil
			}
		}
	}

	if requestItem.ItemType == "book" {
		for i, wishitem := range w.Books {
			if wishitem.Id == requestItem.Id {
				w.Books[i] = requestItem
				return requestItem.ItemType, nil
			}
		}
	}

	if requestItem.ItemType == "mouse-mat" {
		for i, wishitem := range w.Other {
			if wishitem.Id == requestItem.Id {
				w.Other[i] = requestItem
				return requestItem.ItemType, nil
			}
		}
	}

	return "", fmt.Errorf("item not found")
}

type WishItem struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ItemType     string `json:"itemtype"`
	ShopUrl      string `json:"shopurl"`
	WasPurchased bool   `json:"waspurchased"`
	ImgSource    string `json:"imgsource"`
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
	data, err := os.ReadFile("./data/wishlist.json")
	if err != nil {
		fmt.Printf("ERROR %s: %v", "Reading file", err)
	}

	var payload Wishlist
	err = json.Unmarshal(data, &payload)
	if err != nil {
		fmt.Printf("ERROR %s: %v", "Parsing json", err)
	}
	e.GET("/wishlist", func(c echo.Context) error {
		return c.JSON(http.StatusOK, payload)
	})

	//replace the whole wishlist
	e.POST("/wishlist", func(c echo.Context) error {
		requestWishList := new(Wishlist)
		if err := c.Bind(requestWishList); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		payload = *requestWishList

		return c.JSON(http.StatusOK, requestWishList)
	})

	//marks the item as purchased
	e.POST("/wishitem/:id/buy", func(c echo.Context) error {
		id := c.Param("id")

		wishitem := payload.ItemPurchased(id)
		if wishitem == nil {
			erroMsg := fmt.Sprintf("ERROR trying to update file %s: Item not found", id)
			fmt.Println(erroMsg)
			c.String(404, erroMsg)
		}

		return c.Render(200, "wishlistitem", wishitem)
	})

	//update item
	e.POST("/wishitem", func(c echo.Context) error {
		var requestItem WishItem
		if err := c.Bind(&requestItem); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		_, err = payload.UpdateItem(requestItem)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		return c.JSON(http.StatusOK, requestItem)
	})

	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "index", payload)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("WEBSITES_PORT")
	}

	if port == "" {
		port = "43067"
	}

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}

func buildUpdateItemResponse(wishList Wishlist, itemType string) (int, string, interface{}) {
	switch itemType {
	case "t-shirt":
		return 200, "tshirtList", wishList.Tshirts
	case "book":
		return 200, "booksList", wishList.Books
	default:
		return 200, "othersList", wishList.Other
	}
}
