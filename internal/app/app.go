package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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

	e.GET("/", a.getRootHandler)
	e.GET("/wishlist/:slug", a.getListPageHandler)

	e.GET("/wishlist", a.getFullWishListHandler)
	e.GET("/wishlist/refresh", a.refreshFullWishListHandler)
	//replace all lists
	e.POST("/wishlist", a.replaceCompleteWishListHandler)

	//create item within a list
	e.PUT("/wishlist/:slug/wishitem", a.createWishItemHandler)
	//update item within a list
	e.POST("/wishlist/:slug/wishitem", a.updateWishItemHandler)
	//marks the item as purchased within a list
	e.POST("/wishlist/:slug/wishitem/:id/buy", a.purchaseItemHandler)
}

// handleStoreError translates a missing-list error into a 404 instead of a
// generic 500 — a list that doesn't exist is a client error (bad slug), not
// a server failure.
func handleStoreError(err error) error {
	if errors.Is(err, storage.ErrListNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
}

// findList returns the list matching slug, or a zero-value List if absent.
func findList(lists []domain.List, slug string) domain.List {
	for _, l := range lists {
		if l.Slug == slug {
			return l
		}
	}
	return domain.List{}
}

// itemView adds view-only concerns to a WishItem without touching the
// domain type. Embedding keeps every existing template reference (.Id,
// .Title, etc.) working unchanged via field promotion.
type itemView struct {
	domain.WishItem
	HiddenFromViewer bool
	// ListSlug lets the template build the buy button's list-scoped URL
	// without needing access to the page's outer data from within the
	// nested wishlistitem block.
	ListSlug string
}

// newItemView hides a purchased item from the viewer only when they're
// admin AND the list itself is marked as this admin's own recipient list —
// admin browsing someone else's list still sees real purchase status, same
// as any family viewer, so they don't accidentally double-buy for them.
func newItemView(item domain.WishItem, isAdmin bool, list domain.List) itemView {
	return itemView{
		WishItem:         item,
		HiddenFromViewer: isAdmin && item.WasPurchased && list.IsAdminRecipient,
		ListSlug:         list.Slug,
	}
}

// wishlistView reproduces the Tshirts/Books/Other categorization the
// template expects, derived at render time from the flat Items collection.
type wishlistView struct {
	Tshirts     []itemView
	Books       []itemView
	Other       []itemView
	CSRFToken   string
	Lists       []domain.List // for the tab bar; Slug == CurrentSlug marks the active pill
	CurrentSlug string
}

func newWishlistView(w domain.Wishlist, isAdmin bool, list domain.List) wishlistView {
	var view wishlistView
	for _, item := range w.Items {
		iv := newItemView(item, isAdmin, list)
		switch item.ItemType {
		case "t-shirt":
			view.Tshirts = append(view.Tshirts, iv)
		case "book":
			view.Books = append(view.Books, iv)
		default:
			view.Other = append(view.Other, iv)
		}
	}
	return view
}

// listsPayload is the JSON shape for the bulk export/import endpoints,
// matching FileStore's on-disk shape.
type listsPayload struct {
	Lists []domain.ListWithItems `json:"lists"`
}

func (a *app) getRootHandler(c echo.Context) error {
	lists, err := a.store.GetLists()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	slug := ""
	for _, l := range lists {
		if l.IsDefault {
			slug = l.Slug
			break
		}
	}
	if slug == "" && len(lists) > 0 {
		slug = lists[0].Slug
	}
	if slug == "" {
		return echo.NewHTTPError(http.StatusInternalServerError, "no lists configured")
	}

	return c.Redirect(http.StatusFound, "/wishlist/"+slug)
}

func (a *app) getListPageHandler(c echo.Context) error {
	slug := c.Param("slug")

	wishlist, err := a.store.LoadList(slug)
	if err != nil {
		return handleStoreError(err)
	}

	lists, err := a.store.GetLists()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	currentList := findList(lists, slug)

	isAdmin, _ := c.Get("isAdmin").(bool)
	view := newWishlistView(wishlist, isAdmin, currentList)
	view.Lists = lists
	view.CurrentSlug = slug
	if token, ok := c.Get(middleware.DefaultCSRFConfig.ContextKey).(string); ok {
		view.CSRFToken = token
	}

	return c.Render(http.StatusOK, "index", view)
}

func (a *app) createWishItemHandler(c echo.Context) error {
	slug := c.Param("slug")

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

	wishlist, err := a.store.LoadList(slug)
	if err != nil {
		return handleStoreError(err)
	}

	if wishlist.IndexOf(requestItem) == -1 {
		wishlist.AddItem(requestItem)
	} else if _, err := wishlist.UpdateItem(requestItem); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if err := a.store.SaveList(slug, wishlist); err != nil {
		return handleStoreError(err)
	}

	return c.JSON(http.StatusOK, requestItem)
}

func (a *app) updateWishItemHandler(c echo.Context) error {
	slug := c.Param("slug")

	var requestItem domain.WishItem
	if err := c.Bind(&requestItem); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	wishlist, err := a.store.LoadList(slug)
	if err != nil {
		return handleStoreError(err)
	}

	if _, err := wishlist.UpdateItem(requestItem); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if err := a.store.SaveList(slug, wishlist); err != nil {
		return handleStoreError(err)
	}

	return c.JSON(http.StatusOK, requestItem)
}

func (a *app) getFullWishListHandler(c echo.Context) error {
	all, err := a.store.LoadAll()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, listsPayload{Lists: all})
}

func (a *app) refreshFullWishListHandler(c echo.Context) error {
	all, err := a.store.LoadAll()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, listsPayload{Lists: all})
}

func (a *app) replaceCompleteWishListHandler(c echo.Context) error {
	var payload listsPayload
	if err := c.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	seenSlugs := make(map[string]bool, len(payload.Lists))
	for _, l := range payload.Lists {
		if seenSlugs[l.Slug] {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("duplicate list slug %q", l.Slug))
		}
		seenSlugs[l.Slug] = true
	}

	if err := a.store.ReplaceAll(payload.Lists); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, payload)
}

func (a *app) purchaseItemHandler(c echo.Context) error {
	slug := c.Param("slug")
	id := c.Param("id")

	wishlist, err := a.store.LoadList(slug)
	if err != nil {
		return handleStoreError(err)
	}

	//TODO: make the call open a pop-up
	wishitem := wishlist.ItemPurchased(id)
	if wishitem == nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("item %s not found", id))
	}

	if err := a.store.SaveList(slug, wishlist); err != nil {
		return handleStoreError(err)
	}

	currentList, err := a.store.GetList(slug)
	if err != nil {
		return handleStoreError(err)
	}

	isAdmin, _ := c.Get("isAdmin").(bool)
	return c.Render(http.StatusOK, "wishlistitem", newItemView(*wishitem, isAdmin, currentList))
}
