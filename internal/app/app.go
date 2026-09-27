package app

import (
	"context"
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

const dataPath = "./data/wishlist.json"

func (a *app) Init() error {
	if os.Getenv("STORE_TYPE") == "postgres" {
		connString := os.Getenv("DATABASE_URL")
		if connString == "" {
			return fmt.Errorf("STORE_TYPE=postgres requires DATABASE_URL to be set")
		}

		store, err := storage.NewPostgresStore(context.Background(), connString)
		if err != nil {
			return fmt.Errorf("initializing postgres store: %w", err)
		}
		a.store = store
	} else {
		a.store = storage.NewFileStore(dataPath)
	}

	return nil
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

// wishlistView reproduces the Tshirts/Books/Other categorization the
// template expects, derived at render time from the flat Items collection.
type wishlistView struct {
	Tshirts []domain.WishItem
	Books   []domain.WishItem
	Other   []domain.WishItem
}

func newWishlistView(w domain.Wishlist) wishlistView {
	var view wishlistView
	for _, item := range w.Items {
		switch item.ItemType {
		case "t-shirt":
			view.Tshirts = append(view.Tshirts, item)
		case "book":
			view.Books = append(view.Books, item)
		default:
			view.Other = append(view.Other, item)
		}
	}
	return view
}

func (a *app) createWishItemHandler(c echo.Context) error {
	var requestItem domain.WishItem
	if err := c.Bind(&requestItem); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if requestItem.Id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "item has no id")
	}

	if requestItem.ItemType == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "item has no type")
	}

	wishlist, err := a.store.Load()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if wishlist.IndexOf(requestItem) == -1 {
		wishlist.AddItem(requestItem)
	} else if _, err := wishlist.UpdateItem(requestItem); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if err := a.store.SaveWishList(wishlist); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, requestItem)
}

func (a *app) updateWishItemHandler(c echo.Context) error {
	var requestItem domain.WishItem
	if err := c.Bind(&requestItem); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	wishlist, err := a.store.Load()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if _, err := wishlist.UpdateItem(requestItem); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if err := a.store.SaveWishList(wishlist); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, requestItem)
}

func (a *app) getMainPageHandler(c echo.Context) error {
	wishlist, err := a.store.Load()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Render(http.StatusOK, "index", newWishlistView(wishlist))
}

func (a *app) getFullWishListHandler(c echo.Context) error {
	wishlist, err := a.store.Load()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, wishlist)
}

func (a *app) refreshFullWishListHandler(c echo.Context) error {
	wishlist, err := a.store.Load()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, wishlist)
}

func (a *app) replaceCompleteWishListHandler(c echo.Context) error {
	requestWishList := new(domain.Wishlist)
	if err := c.Bind(requestWishList); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := a.store.SaveWishList(*requestWishList); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, requestWishList)
}

func (a *app) purchaseItemHandler(c echo.Context) error {
	id := c.Param("id")

	wishlist, err := a.store.Load()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	//TODO: make the call open a pop-up
	wishitem := wishlist.ItemPurchased(id)
	if wishitem == nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("item %s not found", id))
	}

	if err := a.store.SaveWishList(wishlist); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Render(http.StatusOK, "wishlistitem", wishitem)
}
