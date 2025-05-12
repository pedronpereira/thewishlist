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

var thewishlist domain.Wishlist

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

	thewishlist = a.store.Load()
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

	index := thewishlist.IndexOf(requestItem)

	if index == -1 {
		thewishlist.AddItem(requestItem)
		return c.JSON(http.StatusCreated, requestItem)
	}

	_, err := thewishlist.UpdateItem(requestItem)
	if err != nil {
		fmt.Println(err)
		echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	err = a.store.SaveWishList(thewishlist)
	if err != nil {
		thewishlist = a.store.Load()
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

	_, err := thewishlist.UpdateItem(requestItem)
	if err != nil {
		fmt.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	err = a.store.SaveWishList(thewishlist)
	if err != nil {
		thewishlist = a.store.Load()
		fmt.Println(err)
		echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, requestItem)
}

func (a *app) getMainPageHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "index", thewishlist)
}

func (a *app) getFullWishListHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, thewishlist)
}

func (a *app) refreshFullWishListHandler(c echo.Context) error {
	thewishlist = a.store.Load()

	return c.JSON(http.StatusOK, thewishlist)
}

func (a *app) replaceCompleteWishListHandler(c echo.Context) error {
	requestWishList := new(domain.Wishlist)
	if err := c.Bind(requestWishList); err != nil {
		fmt.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	thewishlist = *requestWishList
	err := a.store.SaveWishList(thewishlist)
	if err != nil {
		fmt.Println(err)
		echo.NewHTTPError(http.StatusInternalServerError, err)

	}
	return c.JSON(http.StatusOK, requestWishList)
}

func (a *app) purchaseItemHandler(c echo.Context) error {
	id := c.Param("id")

	//TODO: make the call open a pop-up
	wishitem := thewishlist.ItemPurchased(id)
	if wishitem == nil {
		erroMsg := fmt.Sprintf("ERROR trying to update file %s: Item not found", id)
		fmt.Println(erroMsg)
		echo.NewHTTPError(http.StatusNotFound, erroMsg)
	}

	err := a.store.SaveWishList(thewishlist)
	if err != nil {
		thewishlist = a.store.Load()
		fmt.Println(err)
		echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.Render(http.StatusOK, "wishlistitem", wishitem)
}
