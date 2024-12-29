package main

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pedronpereira/thewishlist/internal/domain"
	"github.com/pedronpereira/thewishlist/internal/storage"
)

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

var payload domain.Wishlist
var dtProvider storage.Store

const dataPath = "./data/wishlist.json"

func main() {
	e := echo.New()
	e.Use(middleware.Logger())

	e.Static("/css", "css")
	e.Renderer = newTemplate()

	initWishList()

	e.GET("/wishlist", getFullWishListHandler)
	e.GET("/wishlist/refresh", refreshFullWishListHandler)

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

	dbType := os.Getenv("STORE_TYPE")
	if dbType == "cloud" {
		uri := os.Getenv("CONN_STR")
		dbName := os.Getenv("DB_NAME")
		dbCollection := os.Getenv("DB_COLLECTION")
		dtProvider = storage.NewCloudStore(uri, dbName, dbCollection)
	} else {
		dtProvider = storage.NewFileStore(dataPath)
	}

	payload = dtProvider.Load()
}

func createWishItemHandler(c echo.Context) error {
	var requestItem domain.WishItem
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

	err = dtProvider.SaveWishList(payload)
	if err != nil {
		payload = dtProvider.Load()
		fmt.Println(err)
		echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, requestItem)
}

func updateWishItemHandler(c echo.Context) error {
	var requestItem domain.WishItem
	if err := c.Bind(&requestItem); err != nil {
		fmt.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	_, err := payload.UpdateItem(requestItem)
	if err != nil {
		fmt.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	err = dtProvider.SaveWishList(payload)
	if err != nil {
		payload = dtProvider.Load()
		fmt.Println(err)
		echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, requestItem)
}

func getMainPageHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "index", payload)
}

func getFullWishListHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, payload)
}

func refreshFullWishListHandler(c echo.Context) error {
	payload = dtProvider.Load()

	return c.JSON(http.StatusOK, payload)
}

func replaceCompleteWishListHandler(c echo.Context) error {
	requestWishList := new(domain.Wishlist)
	if err := c.Bind(requestWishList); err != nil {
		fmt.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	payload = *requestWishList
	err := dtProvider.SaveWishList(payload)
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

	err := dtProvider.SaveWishList(payload)
	if err != nil {
		payload = dtProvider.Load()
		fmt.Println(err)
		echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.Render(http.StatusOK, "wishlistitem", wishitem)
}
