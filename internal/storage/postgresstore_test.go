package storage

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/pedronpereira/thewishlist/internal/domain"
)

// newTestPostgresStore skips the test unless POSTGRES_TEST_URL is set, and
// wipes the lists/wish_items tables before returning so each test starts
// from a clean slate. NOTE: these tests are unverified by the assistant —
// there is no reachable Postgres in the environment they were written in.
// They mirror the already-verified filestore_test.go cases and the
// production-verified postgresstore.go query patterns, but run this suite
// once against a real (ideally disposable) database before trusting it.
func newTestPostgresStore(t *testing.T) *PostgresStore {
	t.Helper()

	url := os.Getenv("POSTGRES_TEST_URL")
	if url == "" {
		t.Skip("POSTGRES_TEST_URL not set; skipping PostgresStore tests")
	}

	store, err := NewPostgresStore(context.Background(), url)
	if err != nil {
		t.Fatalf("connecting to test postgres: %v", err)
	}

	ctx := context.Background()
	if _, err := store.pool.Exec(ctx, "DELETE FROM wish_items"); err != nil {
		t.Fatalf("clearing wish_items before test: %v", err)
	}
	if _, err := store.pool.Exec(ctx, "DELETE FROM lists"); err != nil {
		t.Fatalf("clearing lists before test: %v", err)
	}

	return store
}

func TestPostgresStore_SaveAndLoadList(t *testing.T) {
	store := newTestPostgresStore(t)

	seed := []domain.ListWithItems{
		{
			List:  domain.List{Slug: "pedro", Name: "Pedro", Icon: "🎁", IsDefault: true},
			Items: []domain.WishItem{{Id: "1", Name: "old", ItemType: "t-shirt"}},
		},
	}
	if err := store.ReplaceAll(seed); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	updated := domain.Wishlist{Items: []domain.WishItem{
		{Id: "1", Name: "new", ItemType: "t-shirt"},
		{Id: "2", Name: "second", ItemType: "book"},
	}}
	if err := store.SaveList("pedro", updated); err != nil {
		t.Fatalf("SaveList() error: %v", err)
	}

	got, err := store.LoadList("pedro")
	if err != nil {
		t.Fatalf("LoadList() error: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("expected 2 items after save, got %d", len(got.Items))
	}
}

func TestPostgresStore_LoadList_NotFound(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	_, err := store.LoadList("missing")
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestPostgresStore_GetList(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{
			List:  domain.List{Slug: "pedro", Name: "Pedro", Icon: "🎁", IsAdminRecipient: true},
			Items: []domain.WishItem{{Id: "1"}, {Id: "2"}},
		},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	got, err := store.GetList("pedro")
	if err != nil {
		t.Fatalf("GetList() error: %v", err)
	}
	if got.Name != "Pedro" || !got.IsAdminRecipient || got.ItemCount != 2 {
		t.Fatalf("expected pedro's own metadata with a count of 2, got %+v", got)
	}
}

func TestPostgresStore_GetList_NotFound(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	_, err := store.GetList("missing")
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestPostgresStore_SaveList_NotFound(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	err := store.SaveList("missing", domain.Wishlist{})
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestPostgresStore_GetLists(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{
			List:  domain.List{Slug: "pedro", Name: "Pedro", Icon: "🎁", IsAdminRecipient: true, IsDefault: true},
			Items: []domain.WishItem{{Id: "1"}, {Id: "2"}, {Id: "3"}},
		},
		{
			List:  domain.List{Slug: "wife", Name: "Wife", Icon: "💝"},
			Items: []domain.WishItem{{Id: "4"}},
		},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	lists, err := store.GetLists()
	if err != nil {
		t.Fatalf("GetLists() error: %v", err)
	}
	if len(lists) != 2 {
		t.Fatalf("expected 2 lists, got %d", len(lists))
	}

	byPedro := lists[0]
	if byPedro.Slug != "pedro" || byPedro.ItemCount != 3 {
		t.Fatalf("expected pedro list with 3 items, got %+v", byPedro)
	}
	if !byPedro.IsAdminRecipient || !byPedro.IsDefault {
		t.Fatalf("expected pedro list flags preserved, got %+v", byPedro)
	}

	byWife := lists[1]
	if byWife.Slug != "wife" || byWife.ItemCount != 1 {
		t.Fatalf("expected wife list with 1 item, got %+v", byWife)
	}
}

func TestPostgresStore_SaveList_PreservesOtherLists(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}, Items: []domain.WishItem{{Id: "1"}}},
		{List: domain.List{Slug: "wife", Name: "Wife"}, Items: []domain.WishItem{{Id: "2"}}},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	if err := store.SaveList("pedro", domain.Wishlist{Items: []domain.WishItem{{Id: "1"}, {Id: "1b"}}}); err != nil {
		t.Fatalf("SaveList() error: %v", err)
	}

	wife, err := store.LoadList("wife")
	if err != nil {
		t.Fatalf("LoadList(wife) error: %v", err)
	}
	if len(wife.Items) != 1 || wife.Items[0].Id != "2" {
		t.Fatalf("expected wife's list untouched by saving pedro's, got %+v", wife.Items)
	}
}

func TestPostgresStore_LoadAllAndReplaceAll(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}, Items: []domain.WishItem{{Id: "1"}}},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	all, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error: %v", err)
	}
	if len(all) != 1 || all[0].Slug != "pedro" {
		t.Fatalf("expected 1 list from LoadAll, got %+v", all)
	}

	replacement := []domain.ListWithItems{
		{List: domain.List{Slug: "a", Name: "A"}, Items: []domain.WishItem{{Id: "x"}}},
		{List: domain.List{Slug: "b", Name: "B"}, Items: []domain.WishItem{{Id: "y"}, {Id: "z"}}},
	}
	if err := store.ReplaceAll(replacement); err != nil {
		t.Fatalf("ReplaceAll() error: %v", err)
	}

	all, err = store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() after ReplaceAll error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 lists after ReplaceAll, got %d", len(all))
	}
	if _, err := store.LoadList("pedro"); !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected the old 'pedro' list to be gone after ReplaceAll, got %v", err)
	}
}

func TestPostgresStore_AddItem(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}, Items: []domain.WishItem{{Id: "1", Name: "existing"}}},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	if err := store.AddItem("pedro", domain.WishItem{Id: "2", Name: "new", ItemType: "book"}); err != nil {
		t.Fatalf("AddItem() error: %v", err)
	}

	got, err := store.LoadList("pedro")
	if err != nil {
		t.Fatalf("LoadList() error: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("expected 2 items after add, got %d: %+v", len(got.Items), got.Items)
	}
}

func TestPostgresStore_AddItem_UpsertsExistingId(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}, Items: []domain.WishItem{{Id: "1", Name: "old", ItemType: "book"}}},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	if err := store.AddItem("pedro", domain.WishItem{Id: "1", Name: "renamed", ItemType: "book"}); err != nil {
		t.Fatalf("AddItem() error: %v", err)
	}

	got, err := store.LoadList("pedro")
	if err != nil {
		t.Fatalf("LoadList() error: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Name != "renamed" {
		t.Fatalf("expected the existing item replaced in place, got %+v", got.Items)
	}
}

func TestPostgresStore_AddItem_NotFound(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	err := store.AddItem("missing", domain.WishItem{Id: "1", ItemType: "book"})
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestPostgresStore_GetItem(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}, Items: []domain.WishItem{{Id: "1", Name: "item1"}}},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	got, err := store.GetItem("pedro", "1")
	if err != nil {
		t.Fatalf("GetItem() error: %v", err)
	}
	if got.Name != "item1" {
		t.Fatalf("expected item1, got %+v", got)
	}

	if _, err := store.GetItem("pedro", "missing"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for a missing item id, got %v", err)
	}
	if _, err := store.GetItem("missing", "1"); !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound for a missing list, got %v", err)
	}
}

func TestPostgresStore_UpdateItem(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{
			List: domain.List{Slug: "pedro", Name: "Pedro"},
			Items: []domain.WishItem{
				{Id: "1", Name: "old", ItemType: "book"},
				{Id: "2", Name: "untouched", ItemType: "t-shirt"},
			},
		},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	if err := store.UpdateItem("pedro", domain.WishItem{Id: "1", Name: "new", ItemType: "book"}); err != nil {
		t.Fatalf("UpdateItem() error: %v", err)
	}

	got, err := store.LoadList("pedro")
	if err != nil {
		t.Fatalf("LoadList() error: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("expected UpdateItem to leave the other item alone, got %+v", got.Items)
	}
	for _, item := range got.Items {
		if item.Id == "1" && item.Name != "new" {
			t.Fatalf("expected item 1 updated, got %+v", item)
		}
		if item.Id == "2" && item.Name != "untouched" {
			t.Fatalf("expected item 2 untouched, got %+v", item)
		}
	}
}

func TestPostgresStore_UpdateItem_NotFound(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}, Items: []domain.WishItem{{Id: "1"}}},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	err := store.UpdateItem("pedro", domain.WishItem{Id: "missing", ItemType: "book"})
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}

	err = store.UpdateItem("missing", domain.WishItem{Id: "1", ItemType: "book"})
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestPostgresStore_DeleteItem(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{
			List:  domain.List{Slug: "pedro", Name: "Pedro"},
			Items: []domain.WishItem{{Id: "1"}, {Id: "2"}},
		},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	if err := store.DeleteItem("pedro", "1"); err != nil {
		t.Fatalf("DeleteItem() error: %v", err)
	}

	got, err := store.LoadList("pedro")
	if err != nil {
		t.Fatalf("LoadList() error: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Id != "2" {
		t.Fatalf("expected only item 2 left, got %+v", got.Items)
	}

	if err := store.DeleteItem("pedro", "1"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound deleting an already-removed item, got %v", err)
	}
	if err := store.DeleteItem("missing", "2"); !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestPostgresStore_PurchaseItem(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.ReplaceAll([]domain.ListWithItems{
		{
			List:  domain.List{Slug: "pedro", Name: "Pedro"},
			Items: []domain.WishItem{{Id: "1", WasPurchased: false}},
		},
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	got, err := store.PurchaseItem("pedro", "1")
	if err != nil {
		t.Fatalf("PurchaseItem() error: %v", err)
	}
	if !got.WasPurchased {
		t.Fatalf("expected the returned item marked purchased, got %+v", got)
	}

	persisted, err := store.LoadList("pedro")
	if err != nil {
		t.Fatalf("LoadList() error: %v", err)
	}
	if !persisted.Items[0].WasPurchased {
		t.Fatalf("expected the purchase to persist, got %+v", persisted.Items[0])
	}

	if _, err := store.PurchaseItem("pedro", "missing"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
	if _, err := store.PurchaseItem("missing", "1"); !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestPostgresStore_CreateList(t *testing.T) {
	store := newTestPostgresStore(t)

	if err := store.CreateList(domain.List{Slug: "natal", Name: "Natal", Icon: "🎄"}); err != nil {
		t.Fatalf("creating list: %v", err)
	}

	got, err := store.GetList("natal")
	if err != nil {
		t.Fatalf("getting created list: %v", err)
	}
	if got.Name != "Natal" || got.Icon != "🎄" {
		t.Fatalf("unexpected created list: %+v", got)
	}

	err = store.CreateList(domain.List{Slug: "natal", Name: "Again"})
	if !errors.Is(err, ErrListExists) {
		t.Fatalf("expected ErrListExists for a duplicate slug, got %v", err)
	}
}
