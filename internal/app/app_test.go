package app

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pedronpereira/thewishlist/internal/domain"
	"github.com/pedronpereira/thewishlist/internal/storage"
)

const (
	testFamilyUser = "family"
	testFamilyPass = "familypass"
	testAdminUser  = "admin"
	testAdminPass  = "adminpass"
)

// testRenderer marshals whatever view struct a handler passes to c.Render
// as JSON, so tests can inspect it without depending on the real HTML
// templates, which live in the separate webapp package.
type testRenderer struct{}

func (r *testRenderer) Render(w io.Writer, name string, data any, c echo.Context) error {
	buf, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = w.Write(buf)
	return err
}

// newTestServer builds a real Echo instance wired the same way
// webapp.New() does — Basic Auth with a family and an admin credential
// pair, CSRF scoped to the list page and buy routes — backed by a real
// FileStore in a temp directory seeded with the given lists. This
// duplicates a small amount of webapp.go's middleware setup rather than
// importing the webapp package, since webapp imports this package (app)
// and app.Init() hardcodes its own data path with no injection point.
func newTestServer(t *testing.T, lists []domain.ListWithItems) *echo.Echo {
	t.Helper()

	path := filepath.Join(t.TempDir(), "wishlist.json")
	store := storage.NewFileStore(path)
	if err := store.ReplaceAll(lists); err != nil {
		t.Fatalf("seeding test store: %v", err)
	}

	a := New()
	a.store = store

	e := echo.New()

	e.Use(middleware.BasicAuth(func(u, p string, c echo.Context) (bool, error) {
		if subtle.ConstantTimeCompare([]byte(u), []byte(testFamilyUser)) == 1 &&
			subtle.ConstantTimeCompare([]byte(p), []byte(testFamilyPass)) == 1 {
			return true, nil
		}
		if subtle.ConstantTimeCompare([]byte(u), []byte(testAdminUser)) == 1 &&
			subtle.ConstantTimeCompare([]byte(p), []byte(testAdminPass)) == 1 {
			c.Set("isAdmin", true)
			return true, nil
		}
		return false, nil
	}))

	e.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
		Skipper: func(c echo.Context) bool {
			switch {
			case c.Request().Method == http.MethodGet && c.Path() == "/wishlist/:slug":
				return false
			case c.Request().Method == http.MethodPost && c.Path() == "/wishlist/:slug/wishitem/:id/buy":
				return false
			case c.Request().Method == http.MethodPut && c.Path() == "/wishlist/:slug/wishitem":
				return false
			case c.Request().Method == http.MethodPost && c.Path() == "/wishlist/:slug/wishitem":
				return false
			case c.Request().Method == http.MethodDelete && c.Path() == "/wishlist/:slug/wishitem/:id":
				return false
			case c.Request().Method == http.MethodGet && c.Path() == "/wishlist/:slug/wishitem/:id/edit":
				return false
			case c.Request().Method == http.MethodPost && c.Path() == "/wishlist/lists":
				return false
			default:
				return true
			}
		},
	}))

	e.Renderer = &testRenderer{}
	a.RegisterHandlers(e)

	return e
}

func doRequest(e *echo.Echo, method, path, user, pass string, body string) *httptest.ResponseRecorder {
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reqBody)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// doProtectedRequest performs a full CSRF round-trip — loading pageSlug's
// list page to obtain a valid token+cookie pair, then issuing the given
// request with both attached — for the routes useCSRFProtection's Skipper
// now covers (create/update/delete item, fetch edit form).
func doProtectedRequest(t *testing.T, e *echo.Echo, pageSlug, method, path, user, pass, body string) *httptest.ResponseRecorder {
	t.Helper()

	pageRec := doRequest(e, http.MethodGet, "/wishlist/"+pageSlug, user, pass, "")
	if pageRec.Code != http.StatusOK {
		t.Fatalf("loading page for CSRF token: got %d: %s", pageRec.Code, pageRec.Body.String())
	}
	cookies := pageRec.Result().Cookies()
	match := csrfTokenRe.FindStringSubmatch(pageRec.Body.String())
	if match == nil {
		t.Fatalf("expected a CSRFToken in the rendered page, got %s", pageRec.Body.String())
	}
	token := match[1]

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reqBody)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.SetBasicAuth(user, pass)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("X-CSRF-Token", token)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func testLists() []domain.ListWithItems {
	return []domain.ListWithItems{
		{
			List: domain.List{Slug: "pedro", Name: "Pedro", Icon: "🎁", IsAdminRecipient: true, IsDefault: true},
			Items: []domain.WishItem{
				{Id: "p1", Name: "item1", ItemType: "t-shirt", WasPurchased: false},
				{Id: "p2", Name: "item2", ItemType: "book", WasPurchased: true},
			},
		},
		{
			List: domain.List{Slug: "wife", Name: "Wife", Icon: "💝", IsAdminRecipient: false},
			Items: []domain.WishItem{
				{Id: "w1", Name: "witem1", ItemType: "other", WasPurchased: true},
			},
		},
	}
}

func TestGetRootHandler_RedirectsToDefaultList(t *testing.T) {
	e := newTestServer(t, testLists())

	rec := doRequest(e, http.MethodGet, "/", testFamilyUser, testFamilyPass, "")

	if rec.Code != http.StatusFound {
		t.Fatalf("expected %d, got %d", http.StatusFound, rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/wishlist/pedro" {
		t.Fatalf("expected redirect to /wishlist/pedro, got %q", loc)
	}
}

func TestGetListPageHandler_UnknownSlugIs404(t *testing.T) {
	e := newTestServer(t, testLists())

	rec := doRequest(e, http.MethodGet, "/wishlist/nonexistent", testFamilyUser, testFamilyPass, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown list, got %d", rec.Code)
	}
}

func TestGetListPageHandler_KnownSlugRenders(t *testing.T) {
	e := newTestServer(t, testLists())

	rec := doRequest(e, http.MethodGet, "/wishlist/pedro", testFamilyUser, testFamilyPass, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"p1"`) {
		t.Fatalf("expected rendered page to include item p1, got %s", rec.Body.String())
	}
}

func TestCreateAndBuyItemWorkflow(t *testing.T) {
	e := newTestServer(t, testLists())

	// Create and update are both admin-only and CSRF-protected now that
	// they're reachable from the browser UI (Phase 3a), so this workflow
	// goes through doProtectedRequest with admin credentials.
	newItem := `{"id":"p3","name":"new","itemtype":"t-shirt","title":"New Item"}`
	rec := doProtectedRequest(t, e, "pedro", http.MethodPut, "/wishlist/pedro/wishitem", testAdminUser, testAdminPass, newItem)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doRequest(e, http.MethodGet, "/wishlist/pedro", testFamilyUser, testFamilyPass, "")
	if !strings.Contains(rec.Body.String(), `"p3"`) {
		t.Fatalf("expected created item p3 to appear on the list page")
	}

	updated := `{"id":"p3","name":"new","itemtype":"t-shirt","title":"New Item","waspurchased":true}`
	rec = doProtectedRequest(t, e, "pedro", http.MethodPost, "/wishlist/pedro/wishitem", testAdminUser, testAdminPass, updated)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doRequest(e, http.MethodGet, "/wishlist/pedro", testFamilyUser, testFamilyPass, "")
	if !strings.Contains(rec.Body.String(), `"waspurchased":true`) {
		t.Fatalf("expected the purchase to persist, got %s", rec.Body.String())
	}
}

// TestDeleteWishItemHandler covers the new delete capability: removing a
// known item persists and returns 200, and deleting an unknown id 404s.
func TestDeleteWishItemHandler(t *testing.T) {
	e := newTestServer(t, testLists())

	rec := doProtectedRequest(t, e, "pedro", http.MethodDelete, "/wishlist/pedro/wishitem/p1", testAdminUser, testAdminPass, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doRequest(e, http.MethodGet, "/wishlist/pedro", testFamilyUser, testFamilyPass, "")
	if strings.Contains(rec.Body.String(), `"p1"`) {
		t.Fatalf("expected deleted item p1 to no longer appear on the list page")
	}

	rec = doProtectedRequest(t, e, "pedro", http.MethodDelete, "/wishlist/pedro/wishitem/nonexistent", testAdminUser, testAdminPass, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete unknown id: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestEditWishItemFormHandler covers the new edit-form fragment route: it
// returns the requested item pre-filled, and 404s for an unknown id.
func TestEditWishItemFormHandler(t *testing.T) {
	e := newTestServer(t, testLists())

	rec := doProtectedRequest(t, e, "pedro", http.MethodGet, "/wishlist/pedro/wishitem/p1/edit", testAdminUser, testAdminPass, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("edit form: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"p1"`) {
		t.Fatalf("expected edit form view to include item p1, got %s", rec.Body.String())
	}

	rec = doProtectedRequest(t, e, "pedro", http.MethodGet, "/wishlist/pedro/wishitem/nonexistent/edit", testAdminUser, testAdminPass, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("edit form unknown id: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminEnforcement_ItemMutations confirms family credentials — valid
// for browsing — are rejected with 403 for every admin-only mutation this
// phase introduces or tightens: create, update, delete, and fetching the
// edit form (a GET, but the entry point to a mutating action).
func TestAdminEnforcement_ItemMutations(t *testing.T) {
	e := newTestServer(t, testLists())

	newItem := `{"title":"New Item"}`
	rec := doProtectedRequest(t, e, "pedro", http.MethodPut, "/wishlist/pedro/wishitem", testFamilyUser, testFamilyPass, newItem)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("create as family: expected 403, got %d: %s", rec.Code, rec.Body.String())
	}

	updated := `{"id":"p1","name":"item1","title":"Renamed"}`
	rec = doProtectedRequest(t, e, "pedro", http.MethodPost, "/wishlist/pedro/wishitem", testFamilyUser, testFamilyPass, updated)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("update as family: expected 403, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doProtectedRequest(t, e, "pedro", http.MethodDelete, "/wishlist/pedro/wishitem/p1", testFamilyUser, testFamilyPass, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("delete as family: expected 403, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doProtectedRequest(t, e, "pedro", http.MethodGet, "/wishlist/pedro/wishitem/p1/edit", testFamilyUser, testFamilyPass, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("edit form as family: expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestReplaceCompleteWishListHandler_RejectsDuplicateSlugs guards against a
// bug a code review caught: FileStore.ReplaceAll had no slug-uniqueness
// check (unlike PostgresStore, protected by a UNIQUE constraint), so a
// payload with two lists sharing a slug would silently orphan the second
// one's items. The fix validates at the handler level, store-agnostic.
func TestReplaceCompleteWishListHandler_RejectsDuplicateSlugs(t *testing.T) {
	e := newTestServer(t, testLists())

	dup := `{"lists":[{"slug":"a","name":"A"},{"slug":"a","name":"A again"}]}`
	rec := doRequest(e, http.MethodPost, "/wishlist", testFamilyUser, testFamilyPass, dup)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate slugs, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestNewItemView_HiddenFromViewer directly tests the core new behavior
// this phase introduces: purchases are hidden from admin only on a list
// marked IsAdminRecipient, never on other lists, and never from non-admin
// viewers regardless of the list.
func TestNewItemView_HiddenFromViewer(t *testing.T) {
	purchased := domain.WishItem{Id: "x", WasPurchased: true}
	unpurchased := domain.WishItem{Id: "y", WasPurchased: false}
	ownList := domain.List{Slug: "pedro", IsAdminRecipient: true}
	otherList := domain.List{Slug: "wife", IsAdminRecipient: false}

	tests := []struct {
		name    string
		item    domain.WishItem
		isAdmin bool
		list    domain.List
		want    bool
	}{
		{"admin, purchased, own list -> hidden", purchased, true, ownList, true},
		{"admin, purchased, other list -> visible", purchased, true, otherList, false},
		{"family, purchased, own list -> visible", purchased, false, ownList, false},
		{"admin, unpurchased, own list -> visible", unpurchased, true, ownList, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newItemView(tt.item, tt.isAdmin, tt.list)
			if got.HiddenFromViewer != tt.want {
				t.Fatalf("HiddenFromViewer = %v, want %v", got.HiddenFromViewer, tt.want)
			}
		})
	}
}

func TestBasicAuth_FamilyAndAdmin(t *testing.T) {
	e := newTestServer(t, testLists())

	rec := doRequest(e, http.MethodGet, "/wishlist/pedro", "", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no credentials: expected 401, got %d", rec.Code)
	}

	rec = doRequest(e, http.MethodGet, "/wishlist/pedro", testFamilyUser, testFamilyPass, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("family credentials: expected 200, got %d", rec.Code)
	}

	rec = doRequest(e, http.MethodGet, "/wishlist/pedro", testAdminUser, testAdminPass, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("admin credentials: expected 200, got %d", rec.Code)
	}

	// Confirms isAdmin was actually set on the request context and reached
	// the view: p2 is purchased on pedro's (IsAdminRecipient) list, so it
	// should be flagged hidden. Whether a hidden item's markup is actually
	// omitted from the page is the real template's job (webapp/views/item.html,
	// verified separately by hand) — this stub renderer just serializes the
	// Go view struct, so the item itself still appears here, correctly
	// flagged. TestNewItemView_HiddenFromViewer covers the flag logic
	// directly and more precisely; this test's job is just the auth wiring.
	var view struct {
		Books []struct {
			Id               string
			HiddenFromViewer bool
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("unmarshaling rendered view: %v", err)
	}
	if len(view.Books) != 1 || view.Books[0].Id != "p2" || !view.Books[0].HiddenFromViewer {
		t.Fatalf("expected p2 flagged HiddenFromViewer for the admin, got %+v", view.Books)
	}
}

var csrfTokenRe = regexp.MustCompile(`"CSRFToken":"([^"]*)"`)

func TestCSRF_BuyFlow(t *testing.T) {
	e := newTestServer(t, testLists())

	rec := doRequest(e, http.MethodGet, "/wishlist/pedro", testFamilyUser, testFamilyPass, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 loading the page, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	match := csrfTokenRe.FindStringSubmatch(rec.Body.String())
	if match == nil {
		t.Fatalf("expected a CSRFToken in the rendered page, got %s", rec.Body.String())
	}
	token := match[1]

	// No cookie, no header: rejected.
	req := httptest.NewRequest(http.MethodPost, "/wishlist/pedro/wishitem/p1/buy", nil)
	req.SetBasicAuth(testFamilyUser, testFamilyPass)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req)
	if rec2.Code == http.StatusOK {
		t.Fatalf("expected buy without a CSRF token to be rejected, got 200")
	}

	// Cookie present, wrong header value: rejected.
	req = httptest.NewRequest(http.MethodPost, "/wishlist/pedro/wishitem/p1/buy", nil)
	req.SetBasicAuth(testFamilyUser, testFamilyPass)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("X-CSRF-Token", "wrong-token")
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req)
	if rec3.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for mismatched CSRF token, got %d", rec3.Code)
	}

	// Cookie + matching header: succeeds.
	req = httptest.NewRequest(http.MethodPost, "/wishlist/pedro/wishitem/p1/buy", nil)
	req.SetBasicAuth(testFamilyUser, testFamilyPass)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("X-CSRF-Token", token)
	rec4 := httptest.NewRecorder()
	e.ServeHTTP(rec4, req)
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200 for a correctly matched CSRF token, got %d: %s", rec4.Code, rec4.Body.String())
	}
}

// TestCSRF_CreateItemFlow mirrors TestCSRF_BuyFlow for the new
// browser-reachable create route: a forged request with no token, or the
// wrong token, must be rejected even with valid admin credentials.
func TestCSRF_CreateItemFlow(t *testing.T) {
	e := newTestServer(t, testLists())

	rec := doRequest(e, http.MethodGet, "/wishlist/pedro", testAdminUser, testAdminPass, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 loading the page, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	match := csrfTokenRe.FindStringSubmatch(rec.Body.String())
	if match == nil {
		t.Fatalf("expected a CSRFToken in the rendered page, got %s", rec.Body.String())
	}
	token := match[1]
	newItem := `{"title":"New Item","itemtype":"t-shirt"}`

	// No cookie, no header: rejected.
	req := httptest.NewRequest(http.MethodPut, "/wishlist/pedro/wishitem", strings.NewReader(newItem))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(testAdminUser, testAdminPass)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req)
	if rec2.Code == http.StatusOK {
		t.Fatalf("expected create without a CSRF token to be rejected, got 200")
	}

	// Cookie present, wrong header value: rejected.
	req = httptest.NewRequest(http.MethodPut, "/wishlist/pedro/wishitem", strings.NewReader(newItem))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(testAdminUser, testAdminPass)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("X-CSRF-Token", "wrong-token")
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req)
	if rec3.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for mismatched CSRF token, got %d", rec3.Code)
	}

	// Cookie + matching header: succeeds.
	req = httptest.NewRequest(http.MethodPut, "/wishlist/pedro/wishitem", strings.NewReader(newItem))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(testAdminUser, testAdminPass)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("X-CSRF-Token", token)
	rec4 := httptest.NewRecorder()
	e.ServeHTTP(rec4, req)
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200 for a correctly matched CSRF token, got %d: %s", rec4.Code, rec4.Body.String())
	}
}

// TestCSRF_DeleteItemFlow mirrors TestCSRF_BuyFlow for the new delete
// route, confirming a destructive action can't be forged cross-site.
func TestCSRF_DeleteItemFlow(t *testing.T) {
	e := newTestServer(t, testLists())

	rec := doRequest(e, http.MethodGet, "/wishlist/pedro", testAdminUser, testAdminPass, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 loading the page, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	match := csrfTokenRe.FindStringSubmatch(rec.Body.String())
	if match == nil {
		t.Fatalf("expected a CSRFToken in the rendered page, got %s", rec.Body.String())
	}
	token := match[1]

	// No cookie, no header: rejected.
	req := httptest.NewRequest(http.MethodDelete, "/wishlist/pedro/wishitem/p1", nil)
	req.SetBasicAuth(testAdminUser, testAdminPass)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req)
	if rec2.Code == http.StatusOK {
		t.Fatalf("expected delete without a CSRF token to be rejected, got 200")
	}

	// Cookie present, wrong header value: rejected.
	req = httptest.NewRequest(http.MethodDelete, "/wishlist/pedro/wishitem/p1", nil)
	req.SetBasicAuth(testAdminUser, testAdminPass)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("X-CSRF-Token", "wrong-token")
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req)
	if rec3.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for mismatched CSRF token, got %d", rec3.Code)
	}

	// Cookie + matching header: succeeds.
	req = httptest.NewRequest(http.MethodDelete, "/wishlist/pedro/wishitem/p1", nil)
	req.SetBasicAuth(testAdminUser, testAdminPass)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("X-CSRF-Token", token)
	rec4 := httptest.NewRecorder()
	e.ServeHTTP(rec4, req)
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200 for a correctly matched CSRF token, got %d: %s", rec4.Code, rec4.Body.String())
	}
}

// TestUpdateWishItemHandler_KeepsNameWhenOmitted guards against an update
// blanking an item's internal name: UpdateItem replaces the whole item, so a
// request without name must keep the stored one.
func TestUpdateWishItemHandler_KeepsNameWhenOmitted(t *testing.T) {
	e := newTestServer(t, testLists())

	updated := `{"id":"p1","title":"Renamed","itemtype":"t-shirt"}`
	rec := doProtectedRequest(t, e, "pedro", http.MethodPost, "/wishlist/pedro/wishitem", testAdminUser, testAdminPass, updated)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doRequest(e, http.MethodGet, "/wishlist", testAdminUser, testAdminPass, "")
	var export struct {
		Lists []struct {
			Slug  string `json:"slug"`
			Items []struct {
				Id    string `json:"id"`
				Name  string `json:"name"`
				Title string `json:"title"`
			} `json:"items"`
		} `json:"lists"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &export); err != nil {
		t.Fatalf("unmarshaling export: %v", err)
	}
	for _, l := range export.Lists {
		for _, item := range l.Items {
			if item.Id == "p1" {
				if item.Title != "Renamed" || item.Name != "item1" {
					t.Fatalf("expected title Renamed and name item1 kept, got title %q name %q", item.Title, item.Name)
				}
				return
			}
		}
	}
	t.Fatalf("item p1 not found in export")
}

func TestCreateListHandler_CreatesListAndRedirects(t *testing.T) {
	e := newTestServer(t, testLists())

	body := `{"name":"Natal 2026","icon":"🎄"}`
	rec := doProtectedRequest(t, e, "pedro", http.MethodPost, "/wishlist/lists", testAdminUser, testAdminPass, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("HX-Redirect"); loc != "/wishlist/natal-2026" {
		t.Fatalf("expected HX-Redirect to /wishlist/natal-2026, got %q", loc)
	}

	rec = doRequest(e, http.MethodGet, "/wishlist/natal-2026", testFamilyUser, testFamilyPass, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("new list page: expected 200, got %d", rec.Code)
	}
}

func TestCreateListHandler_DefaultsIconWhenBlank(t *testing.T) {
	e := newTestServer(t, testLists())

	rec := doProtectedRequest(t, e, "pedro", http.MethodPost, "/wishlist/lists", testAdminUser, testAdminPass, `{"name":"Viagem"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doRequest(e, http.MethodGet, "/wishlist", testFamilyUser, testFamilyPass, "")
	if !strings.Contains(rec.Body.String(), `"icon":"🎁"`) {
		t.Fatalf("expected the new list to default to the 🎁 icon, got %s", rec.Body.String())
	}
}

func TestCreateListHandler_RejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"empty name", `{"name":"   "}`, http.StatusBadRequest},
		{"name with no letters or digits", `{"name":"!!!"}`, http.StatusBadRequest},
		{"reserved slug", `{"name":"Refresh"}`, http.StatusBadRequest},
		{"duplicate of existing list", `{"name":"Pedro"}`, http.StatusConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestServer(t, testLists())
			rec := doProtectedRequest(t, e, "pedro", http.MethodPost, "/wishlist/lists", testAdminUser, testAdminPass, tt.body)
			if rec.Code != tt.want {
				t.Fatalf("expected %d, got %d: %s", tt.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCreateListHandler_FamilyIsForbidden(t *testing.T) {
	e := newTestServer(t, testLists())

	rec := doProtectedRequest(t, e, "pedro", http.MethodPost, "/wishlist/lists", testFamilyUser, testFamilyPass, `{"name":"Natal"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for family credentials, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCSRF_CreateListFlow(t *testing.T) {
	e := newTestServer(t, testLists())

	rec := doRequest(e, http.MethodGet, "/wishlist/pedro", testAdminUser, testAdminPass, "")
	cookies := rec.Result().Cookies()

	req := httptest.NewRequest(http.MethodPost, "/wishlist/lists", strings.NewReader(`{"name":"Natal"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(testAdminUser, testAdminPass)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req)
	if rec2.Code == http.StatusOK {
		t.Fatalf("expected create without a CSRF token to be rejected, got 200")
	}

	match := csrfTokenRe.FindStringSubmatch(rec.Body.String())
	if match == nil {
		t.Fatalf("expected a CSRFToken in the rendered page")
	}
	req = httptest.NewRequest(http.MethodPost, "/wishlist/lists", strings.NewReader(`{"name":"Natal"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(testAdminUser, testAdminPass)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("X-CSRF-Token", match[1])
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 with a matching CSRF token, got %d: %s", rec3.Code, rec3.Body.String())
	}
}

func TestDeriveListSlug(t *testing.T) {
	tests := map[string]string{
		"Natal 2026":            "natal-2026",
		"  Lista de Família!! ": "lista-de-familia",
		"Pedro's wife":          "pedro-s-wife",
		"Ç":                     "c",
		"---":                   "",
	}
	for name, want := range tests {
		if got := deriveListSlug(name); got != want {
			t.Errorf("deriveListSlug(%q) = %q, want %q", name, got, want)
		}
	}
}

// listPageView mirrors wishlistView's fields as the test renderer serializes
// them (Go field names, no JSON tags).
type listPageView struct {
	Tshirts     []struct{ Id string }
	Books       []struct{ Id string }
	Other       []struct{ Id string }
	Filter      string
	AllCount    int
	TshirtCount int
	BookCount   int
	OtherCount  int
}

func TestListPageFilter(t *testing.T) {
	e := newTestServer(t, testLists())

	tests := []struct {
		name       string
		query      string
		wantTshirt int
		wantBook   int
		wantOther  int
	}{
		{"no filter shows everything", "", 1, 1, 0},
		{"book filter", "?tipo=book", 0, 1, 0},
		{"t-shirt filter", "?tipo=t-shirt", 1, 0, 0},
		{"outros filter", "?tipo=outros", 0, 0, 0},
		{"unknown filter is ignored", "?tipo=bogus", 1, 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doRequest(e, http.MethodGet, "/wishlist/pedro"+tt.query, testFamilyUser, testFamilyPass, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}
			var view listPageView
			if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
				t.Fatalf("unmarshaling view: %v", err)
			}
			if len(view.Tshirts) != tt.wantTshirt || len(view.Books) != tt.wantBook || len(view.Other) != tt.wantOther {
				t.Fatalf("shown t-shirts/books/other = %d/%d/%d, want %d/%d/%d",
					len(view.Tshirts), len(view.Books), len(view.Other), tt.wantTshirt, tt.wantBook, tt.wantOther)
			}
			if view.AllCount != 2 || view.TshirtCount != 1 || view.BookCount != 1 || view.OtherCount != 0 {
				t.Fatalf("counts should describe the whole list, got all=%d tshirt=%d book=%d other=%d",
					view.AllCount, view.TshirtCount, view.BookCount, view.OtherCount)
			}
		})
	}
}
