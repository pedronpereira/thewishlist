package app

import (
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/pedronpereira/thewishlist/internal/domain"
	"github.com/pedronpereira/thewishlist/internal/storage"
)

type app struct {
	store storage.Store
}

func New() *app {
	return &app{}
}

var payload domain.Wishlist

const dataPath = "./data/wishlist.json"

func (a *app) Init() {
	dbType := os.Getenv("STORE_TYPE")
	if dbType == "cloud" {
		uri := os.Getenv("CONN_STR")
		dbName := os.Getenv("DB_NAME")
		dbCollection := os.Getenv("DB_COLLECTION")
		a.store = storage.NewCloudStore(uri, dbName, dbCollection)
	} else {
		a.store = storage.NewFileStore(dataPath)
	}

	payload = a.store.Load()
}

func (a *app) RegisterHandlers(e *echo.Echo) {

	e.GET("/", a.getMainPageHandler)

	e.GET("/wishlist", a.getFullWishListHandler)
	e.GET("/wishlist/refresh", a.refreshFullWishListHandler)
	//replace the whole wishlist
	e.POST("/wishlist", a.replaceCompleteWishListHandler)

	//create item
	e.PUT("/wishitem", a.createWishItemHandler)
	//update item
	e.POST("/wishitem", a.updateWishItemHandler)
	//marks the item as purchased
	e.POST("/wishitem/:id/buy", a.purchaseItemHandler)
}

func (a *app) createWishItemHandler(c echo.Context) error {
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

	err = a.store.SaveWishList(payload)
	if err != nil {
		payload = a.store.Load()
		fmt.Println(err)
		echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, requestItem)
}

func (a *app) updateWishItemHandler(c echo.Context) error {
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

	err = a.store.SaveWishList(payload)
	if err != nil {
		payload = a.store.Load()
		fmt.Println(err)
		echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, requestItem)
}

func (a *app) getMainPageHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "index", payload)
}

func (a *app) getFullWishListHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, payload)
}

func (a *app) refreshFullWishListHandler(c echo.Context) error {
	payload = a.store.Load()

	return c.JSON(http.StatusOK, payload)
}

func (a *app) replaceCompleteWishListHandler(c echo.Context) error {
	requestWishList := new(domain.Wishlist)
	if err := c.Bind(requestWishList); err != nil {
		fmt.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	payload = *requestWishList
	err := a.store.SaveWishList(payload)
	if err != nil {
		fmt.Println(err)
		echo.NewHTTPError(http.StatusInternalServerError, err)

	}
	return c.JSON(http.StatusOK, requestWishList)
}

func (a *app) purchaseItemHandler(c echo.Context) error {
	id := c.Param("id")

	wishitem := payload.ItemPurchased(id)
	if wishitem == nil {
		erroMsg := fmt.Sprintf("ERROR trying to update file %s: Item not found", id)
		fmt.Println(erroMsg)
		echo.NewHTTPError(http.StatusNotFound, erroMsg)
	}

	err := a.store.SaveWishList(payload)
	if err != nil {
		payload = a.store.Load()
		fmt.Println(err)
		echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.Render(http.StatusOK, "wishlistitem", wishitem)
}
