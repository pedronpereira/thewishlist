package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"slices"

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

func (w *Wishlist) IndexOf(item WishItem) int {
	index := -1
	switch item.ItemType {
	case "t-shirt":
		index = slices.IndexFunc(w.Tshirts, func(i WishItem) bool {
			return i.Id == item.Id
		})
	case "book":
		index = slices.IndexFunc(w.Books, func(i WishItem) bool {
			return i.Id == item.Id
		})
	default:
		index = slices.IndexFunc(w.Other, func(i WishItem) bool {
			return i.Id == item.Id
		})
	}

	return index
}

func (w *Wishlist) AddItem(item WishItem) {
	switch item.ItemType {
	case "t-shirt":
		w.Tshirts = append(w.Tshirts, item)
	case "book":
		w.Books = append(w.Books, item)
	default:
		w.Other = append(w.Other, item)
	}
}

func (w *Wishlist) GetItem(itemType string, index int) WishItem {
	switch itemType {
	case "t-shirt":
		return w.Tshirts[index]
	case "book":
		return w.Books[index]
	default:
		return w.Other[index]
	}
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

var payload Wishlist

const dataPath = "./data/wishlist.json"

func main() {
	e := echo.New()
	e.Use(middleware.Logger())

	e.Static("/css", "css")
	e.Renderer = newTemplate()

	initWishList()

	e.GET("/wishlist", getFullWishListHandler)

	//replace the whole wishlist
	e.POST("/wishlist", replaceCompleteWishListHandler)

	//marks the item as purchased
	e.POST("/wishitem/:id/buy", purchaseItemHandler)

	//update item
	e.POST("/wishitem", updateWishItemHandler)

	//create item
	e.PUT("/wishitem", createWishItemHandler)

	e.GET("/", getMainPageHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("WEBSITES_PORT")
	}

	if port == "" {
		port = "43067"
	}

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}

func initWishList() {
	//load the file
	data, err := os.ReadFile(dataPath)
	if err != nil {
		fmt.Printf("ERROR %s: %v", "Reading file", err)
	}

	err = json.Unmarshal(data, &payload)
	if err != nil {
		fmt.Printf("ERROR %s: %v", "Parsing json", err)
	}
}

func createWishItemHandler(c echo.Context) error {
	var requestItem WishItem
	if err := c.Bind(&requestItem); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if requestItem.Id == "" {
		return fmt.Errorf("item has no id")
	}

	if requestItem.ItemType == "" {
		return fmt.Errorf("item has no type")
	}

	index := payload.IndexOf(requestItem)

	if index == -1 {
		payload.AddItem(requestItem)
		return c.JSON(http.StatusCreated, requestItem)
	}

	_, err := payload.UpdateItem(requestItem)
	if err != nil {
		fmt.Println(err)
		echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, requestItem)
}

func updateWishItemHandler(c echo.Context) error {
	var requestItem WishItem
	if err := c.Bind(&requestItem); err != nil {
		fmt.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	_, err := payload.UpdateItem(requestItem)
	if err != nil {
		fmt.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, requestItem)
}

func getMainPageHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "index", payload)
}

func getFullWishListHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, payload)
}

func replaceCompleteWishListHandler(c echo.Context) error {
	requestWishList := new(Wishlist)
	if err := c.Bind(requestWishList); err != nil {
		fmt.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	payload = *requestWishList
	err := saveWishList()
	if err != nil {
		fmt.Println(err)
		echo.NewHTTPError(http.StatusInternalServerError, err)

	}
	return c.JSON(http.StatusOK, requestWishList)
}

func purchaseItemHandler(c echo.Context) error {
	id := c.Param("id")

	wishitem := payload.ItemPurchased(id)
	if wishitem == nil {
		erroMsg := fmt.Sprintf("ERROR trying to update file %s: Item not found", id)
		fmt.Println(erroMsg)
		echo.NewHTTPError(http.StatusNotFound, erroMsg)
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		erroMsg := fmt.Sprintf("ERROR trying to marshal wishlist %s", err)
		wishitem.WasPurchased = !wishitem.WasPurchased

		_, err = payload.UpdateItem(*wishitem)
		if err != nil {
			erroMsg = fmt.Sprintf("ERROR Trying to update item AFTER marshalling %s", err)
			fmt.Println(erroMsg)
			return echo.NewHTTPError(http.StatusInternalServerError, erroMsg)
		}

		fmt.Println(erroMsg)
		echo.NewHTTPError(http.StatusInternalServerError, erroMsg)
	}

	err = os.WriteFile(dataPath, buf, 0644)
	if err != nil {
		erroMsg := fmt.Sprintf("ERROR trying to update file %s", err)
		fmt.Println(erroMsg)
		echo.NewHTTPError(http.StatusInternalServerError, erroMsg)
	}

	return c.Render(http.StatusOK, "wishlistitem", wishitem)
}

func saveWishList() error {
	buf, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ERROR trying to marshal wishlist %s", err)
	}

	err = os.WriteFile(dataPath, buf, 0644)
	if err != nil {
		return fmt.Errorf("ERROR trying to update file %s", err)
	}

	return nil
}
