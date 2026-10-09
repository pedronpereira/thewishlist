package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"unicode"

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
	//admin-only: pre-filled edit form fragment for one item
	e.GET("/wishlist/:slug/wishitem/:id/edit", a.editWishItemFormHandler)
	//admin-only: remove an item from a list
	e.DELETE("/wishlist/:slug/wishitem/:id", a.deleteWishItemHandler)
	//admin-only: create an empty list
	e.POST("/wishlist/lists", a.createListHandler)
	//admin-only: look up an og:image for the add-item form's shop link
	e.GET("/wishlist/fetch-image", a.fetchItemImageHandler)
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

// requireAdmin rejects a request with 403 unless it was authenticated with
// the admin credential pair (see useBasicAuth in webapp/webapp.go). Item
// mutations are admin-only — previously any family-password holder could
// call these endpoints directly since no such check existed; that gap is
// closed here now that these actions have a real UI.
func requireAdmin(c echo.Context) error {
	isAdmin, _ := c.Get("isAdmin").(bool)
	if !isAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "admin access required")
	}
	return nil
}

// generateItemID returns a random UUID-v4-shaped string, used so the
// add-item form never needs the admin to invent a unique id by hand.
func generateItemID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating item id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// deriveItemName lowercases title and strips everything but letters/digits,
// matching the existing data convention (e.g. "Mushroom Graffiti" ->
// "mushroomgraffiti") — used so the add-item form doesn't need a separate
// "Name" field, since it's never shown in any template anyway.
func deriveItemName(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// itemView adds view-only concerns to a WishItem without touching the
// domain type. Embedding keeps every existing template reference (.Id,
// .Title, etc.) working unchanged via field promotion.
type itemView struct {
	domain.WishItem
	// ListSlug lets the template build the buy/edit/remove button URLs
	// without needing access to the page's outer data from within the
	// nested wishlistitem block.
	ListSlug string
	// IsAdmin controls whether the edit/remove controls render at all.
	IsAdmin bool
}

func newItemView(item domain.WishItem, isAdmin bool, list domain.List) itemView {
	return itemView{WishItem: item, ListSlug: list.Slug, IsAdmin: isAdmin}
}

// itemTypeFallback stands in for an item with no ItemType set. The forms
// require one, so this should never happen in practice; it just keeps the
// filter menu well-defined if it ever does.
const itemTypeFallback = "sem-tipo"

// titleCaseItemType turns a free-text ItemType into a filter-menu label
// ("t-shirt" -> "T-Shirt", "boardgame" -> "Boardgame"). ItemType has no
// fixed set of values; the admin can type anything into the item form's
// "Tipo" field, so this derives a label from whatever's actually there
// instead of maintaining a translation table that would need updating
// every time a new type shows up.
func titleCaseItemType(s string) string {
	var b strings.Builder
	newWord := true
	for _, r := range s {
		if r == '-' || r == ' ' {
			b.WriteRune(r)
			newWord = true
			continue
		}
		if newWord {
			b.WriteRune(unicode.ToUpper(r))
			newWord = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// typeCount is one row of the filter menu: a real ItemType value present in
// the list, its display label, and how many items have it.
type typeCount struct {
	Type  string
	Label string
	Count int
}

// buildTypeCounts turns a type-to-count tally into a slice sorted by label.
// Sorted explicitly here, rather than relying on html/template's own
// stable-but-easy-to-miss habit of sorting map keys when ranging over a
// map, so the order is obvious from the Go code and testable without going
// through the template at all.
func buildTypeCounts(counts map[string]int) []typeCount {
	result := make([]typeCount, 0, len(counts))
	for t, n := range counts {
		result = append(result, typeCount{Type: t, Label: titleCaseItemType(t), Count: n})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Label < result[j].Label })
	return result
}

// wishlistView is the flat item list for one page render, derived at
// render time from the Wishlist's Items collection.
type wishlistView struct {
	Items       []itemView
	TypeCounts  []typeCount // for the filter menu, sorted by Label
	CSRFToken   string
	Lists       []domain.List // for the tab bar; Slug == CurrentSlug marks the active pill
	CurrentSlug string
	IsAdmin     bool // controls the admin-only list bar buttons
	Filter      string
	Revealed    bool
	AllCount    int // items actually in scope for this render, before the type filter
	HiddenCount int // items hidden from the admin on their own list, regardless of Revealed
}

// newWishlistView builds the page's item list. A purchased item is left out
// entirely, not just visually hidden, when the viewer is the admin on their
// own IsAdminRecipient list, unless reveal is set: admin browsing any other
// list sees real purchase status like any family viewer, so they don't
// accidentally double-buy for someone else. When filter is set, only items
// of that exact ItemType are included; AllCount and TypeCounts still
// describe everything in scope (post-reveal, pre-filter) so the menus can
// show accurate totals regardless of the current filter.
func newWishlistView(w domain.Wishlist, isAdmin bool, list domain.List, filter string, reveal bool) wishlistView {
	view := wishlistView{IsAdmin: isAdmin, Filter: filter, Revealed: reveal}
	counts := map[string]int{}
	for _, item := range w.Items {
		hiddenFromViewer := isAdmin && item.WasPurchased && list.IsAdminRecipient
		if hiddenFromViewer {
			view.HiddenCount++
			if !reveal {
				continue
			}
		}

		view.AllCount++
		itemType := item.ItemType
		if itemType == "" {
			itemType = itemTypeFallback
		}
		counts[itemType]++

		if filter != "" && filter != itemType {
			continue
		}
		view.Items = append(view.Items, newItemView(item, isAdmin, list))
	}
	view.TypeCounts = buildTypeCounts(counts)
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
	reveal := isAdmin && c.QueryParam("revelar") != ""
	view := newWishlistView(wishlist, isAdmin, currentList, c.QueryParam("tipo"), reveal)
	view.Lists = lists
	view.CurrentSlug = slug
	if token, ok := c.Get(middleware.DefaultCSRFConfig.ContextKey).(string); ok {
		view.CSRFToken = token
	}

	return c.Render(http.StatusOK, "index", view)
}

func (a *app) createWishItemHandler(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}

	slug := c.Param("slug")

	var requestItem domain.WishItem
	if err := c.Bind(&requestItem); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if requestItem.ItemType == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "item has no type")
	}

	// The add-item form never sends an id or name — generated here instead
	// of asking the admin to invent a unique id by hand. Explicit curl/API
	// callers that already set these keep working unchanged.
	if requestItem.Id == "" {
		id, err := generateItemID()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		requestItem.Id = id
	}
	if requestItem.Name == "" {
		requestItem.Name = deriveItemName(requestItem.Title)
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

// fetchItemImageHandler backs the add-item form's "Buscar imagem" button:
// given a shop URL, it fetches that page server-side and returns the
// og:image (or twitter:image) it finds, so the admin doesn't have to open
// the link and copy an image URL by hand. It doesn't touch storage, so it's
// a plain GET with no CSRF token required (see useCSRFProtection).
func (a *app) fetchItemImageHandler(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}

	rawURL := c.QueryParam("url")
	if rawURL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "indica um link da loja")
	}

	imageURL, err := fetchOpenGraphImage(rawURL)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]string{"imageUrl": imageURL})
}

func (a *app) updateWishItemHandler(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}

	slug := c.Param("slug")

	var requestItem domain.WishItem
	if err := c.Bind(&requestItem); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	wishlist, err := a.store.LoadList(slug)
	if err != nil {
		return handleStoreError(err)
	}

	// UpdateItem replaces the whole item, so a request that omits name
	// would otherwise blank it. Keep the stored name in that case.
	if requestItem.Name == "" {
		if index := wishlist.IndexOf(requestItem); index != -1 {
			requestItem.Name = wishlist.Items[index].Name
		}
	}

	if _, err := wishlist.UpdateItem(requestItem); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if err := a.store.SaveList(slug, wishlist); err != nil {
		return handleStoreError(err)
	}

	return c.JSON(http.StatusOK, requestItem)
}

// reservedListSlugs are routes already claimed under /wishlist/, so a list
// with one of these slugs could never be opened.
var reservedListSlugs = map[string]bool{
	"refresh": true,
	"lists":   true,
}

var accentReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c",
)

// deriveListSlug turns a list name into a URL segment: lowercase ASCII
// letters and digits, with runs of anything else collapsed to a single
// hyphen. "Natal 2026 — Família" becomes "natal-2026-familia".
func deriveListSlug(name string) string {
	var b strings.Builder
	pendingHyphen := false
	for _, r := range accentReplacer.Replace(strings.ToLower(name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			pendingHyphen = false
		} else {
			pendingHyphen = true
		}
	}
	return b.String()
}

func (a *app) createListHandler(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}

	var request struct {
		Name          string `json:"name"`
		Icon          string `json:"icon"`
		HideFromAdmin bool   `json:"hidefromadmin"`
	}
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Indica um nome para a lista.")
	}

	slug := deriveListSlug(name)
	if slug == "" || reservedListSlugs[slug] {
		return echo.NewHTTPError(http.StatusBadRequest, "Este nome não pode ser usado para uma lista.")
	}

	icon := strings.TrimSpace(request.Icon)
	if icon == "" {
		icon = "🎁"
	}

	list := domain.List{
		Slug:             slug,
		Name:             name,
		Icon:             icon,
		IsAdminRecipient: request.HideFromAdmin,
	}
	if err := a.store.CreateList(list); err != nil {
		if errors.Is(err, storage.ErrListExists) {
			return echo.NewHTTPError(http.StatusConflict, "Já existe uma lista com esse nome.")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	c.Response().Header().Set("HX-Redirect", "/wishlist/"+slug)
	return c.NoContent(http.StatusOK)
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
	if isAdmin && wishitem.WasPurchased && currentList.IsAdminRecipient {
		// The buyer just unknowingly bought and hid their own gift. Mirrors
		// deleteWishItemHandler: 200 with an empty body, not 204, so HTMX
		// actually performs the morph swap and the card disappears; a 204
		// makes HTMX skip the swap entirely.
		return c.NoContent(http.StatusOK)
	}
	return c.Render(http.StatusOK, "wishlistitem", newItemView(*wishitem, isAdmin, currentList))
}

// itemFormView is the render data for the admin-only edit-item form
// fragment — always admin-only by virtue of the route requiring it, so no
// IsAdmin field is needed here unlike itemView.
type itemFormView struct {
	domain.WishItem
	ListSlug string
}

func (a *app) editWishItemFormHandler(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}

	slug := c.Param("slug")
	id := c.Param("id")

	wishlist, err := a.store.LoadList(slug)
	if err != nil {
		return handleStoreError(err)
	}

	index := wishlist.IndexOf(domain.WishItem{Id: id})
	if index == -1 {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("item %s not found", id))
	}

	return c.Render(http.StatusOK, "wishitemeditform", itemFormView{
		WishItem: wishlist.Items[index],
		ListSlug: slug,
	})
}

func (a *app) deleteWishItemHandler(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}

	slug := c.Param("slug")
	id := c.Param("id")

	wishlist, err := a.store.LoadList(slug)
	if err != nil {
		return handleStoreError(err)
	}

	if !wishlist.RemoveItem(id) {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("item %s not found", id))
	}

	if err := a.store.SaveList(slug, wishlist); err != nil {
		return handleStoreError(err)
	}

	// 200 (not 204) with an empty body: htmx skips the swap entirely for
	// 204, which would leave the card in the DOM instead of removing it.
	return c.NoContent(http.StatusOK)
}
