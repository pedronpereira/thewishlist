package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pedronpereira/thewishlist/internal/domain"
)

func newTestFileStore(t *testing.T, lists []domain.ListWithItems) *FileStore {
	t.Helper()

	path := filepath.Join(t.TempDir(), "wishlist.json")
	fs := &FileStore{path: path}
	if err := fs.ReplaceAll(lists); err != nil {
		t.Fatalf("seeding test file store: %v", err)
	}
	return fs
}

func TestFileStore_SaveAndLoadList(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{
			List: domain.List{Slug: "pedro", Name: "Pedro", Icon: "🎁", IsDefault: true},
			Items: []domain.WishItem{
				{Id: "1", Name: "old", ItemType: "t-shirt"},
			},
		},
	})

	updated := domain.Wishlist{Items: []domain.WishItem{
		{Id: "1", Name: "new", ItemType: "t-shirt"},
		{Id: "2", Name: "second", ItemType: "book"},
	}}
	if err := fs.SaveList("pedro", updated); err != nil {
		t.Fatalf("SaveList() error: %v", err)
	}

	got, err := fs.LoadList("pedro")
	if err != nil {
		t.Fatalf("LoadList() error: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("expected 2 items after save, got %d", len(got.Items))
	}
	if got.Items[0].Name != "new" {
		t.Fatalf("expected round-tripped item to reflect the save, got %+v", got.Items[0])
	}
}

func TestFileStore_LoadList_NotFound(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}},
	})

	_, err := fs.LoadList("missing")
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestFileStore_GetList(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{
			List:  domain.List{Slug: "pedro", Name: "Pedro", Icon: "🎁", IsAdminRecipient: true},
			Items: []domain.WishItem{{Id: "1"}, {Id: "2"}},
		},
	})

	got, err := fs.GetList("pedro")
	if err != nil {
		t.Fatalf("GetList() error: %v", err)
	}
	if got.Name != "Pedro" || !got.IsAdminRecipient || got.ItemCount != 2 {
		t.Fatalf("expected pedro's own metadata with a count of 2, got %+v", got)
	}
}

func TestFileStore_GetList_NotFound(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}},
	})

	_, err := fs.GetList("missing")
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestFileStore_SaveList_NotFound(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}},
	})

	err := fs.SaveList("missing", domain.Wishlist{})
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestFileStore_GetLists(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{
			List:  domain.List{Slug: "pedro", Name: "Pedro", Icon: "🎁", IsAdminRecipient: true, IsDefault: true},
			Items: []domain.WishItem{{Id: "1"}, {Id: "2"}, {Id: "3"}},
		},
		{
			List:  domain.List{Slug: "wife", Name: "Wife", Icon: "💝"},
			Items: []domain.WishItem{{Id: "4"}},
		},
	})

	lists, err := fs.GetLists()
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

func TestFileStore_SaveList_PreservesOtherLists(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{
			List:  domain.List{Slug: "pedro", Name: "Pedro"},
			Items: []domain.WishItem{{Id: "1"}},
		},
		{
			List:  domain.List{Slug: "wife", Name: "Wife"},
			Items: []domain.WishItem{{Id: "2"}},
		},
	})

	if err := fs.SaveList("pedro", domain.Wishlist{Items: []domain.WishItem{{Id: "1"}, {Id: "1b"}}}); err != nil {
		t.Fatalf("SaveList() error: %v", err)
	}

	wife, err := fs.LoadList("wife")
	if err != nil {
		t.Fatalf("LoadList(wife) error: %v", err)
	}
	if len(wife.Items) != 1 || wife.Items[0].Id != "2" {
		t.Fatalf("expected wife's list untouched by saving pedro's, got %+v", wife.Items)
	}
}

func TestFileStore_LoadAllAndReplaceAll(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}, Items: []domain.WishItem{{Id: "1"}}},
	})

	all, err := fs.LoadAll()
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
	if err := fs.ReplaceAll(replacement); err != nil {
		t.Fatalf("ReplaceAll() error: %v", err)
	}

	all, err = fs.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() after ReplaceAll error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 lists after ReplaceAll, got %d", len(all))
	}
	if _, err := fs.LoadList("pedro"); !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected the old 'pedro' list to be gone after ReplaceAll, got %v", err)
	}
}

func TestFileStore_CreateList(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro", IsDefault: true}},
	})

	if err := fs.CreateList(domain.List{Slug: "natal", Name: "Natal", Icon: "🎄"}); err != nil {
		t.Fatalf("creating list: %v", err)
	}

	got, err := fs.GetList("natal")
	if err != nil {
		t.Fatalf("getting created list: %v", err)
	}
	if got.Name != "Natal" || got.Icon != "🎄" || got.ItemCount != 0 {
		t.Fatalf("unexpected created list: %+v", got)
	}

	err = fs.CreateList(domain.List{Slug: "natal", Name: "Again"})
	if !errors.Is(err, ErrListExists) {
		t.Fatalf("expected ErrListExists for a duplicate slug, got %v", err)
	}
}

func TestFileStore_AddItem(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}, Items: []domain.WishItem{{Id: "1", Name: "existing"}}},
	})

	if err := fs.AddItem("pedro", domain.WishItem{Id: "2", Name: "new", ItemType: "book"}); err != nil {
		t.Fatalf("AddItem() error: %v", err)
	}

	got, err := fs.LoadList("pedro")
	if err != nil {
		t.Fatalf("LoadList() error: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("expected 2 items after add, got %d: %+v", len(got.Items), got.Items)
	}
}

func TestFileStore_AddItem_UpsertsExistingId(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}, Items: []domain.WishItem{{Id: "1", Name: "old", ItemType: "book"}}},
	})

	if err := fs.AddItem("pedro", domain.WishItem{Id: "1", Name: "renamed", ItemType: "book"}); err != nil {
		t.Fatalf("AddItem() error: %v", err)
	}

	got, err := fs.LoadList("pedro")
	if err != nil {
		t.Fatalf("LoadList() error: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Name != "renamed" {
		t.Fatalf("expected the existing item replaced in place, got %+v", got.Items)
	}
}

func TestFileStore_AddItem_NotFound(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}},
	})

	err := fs.AddItem("missing", domain.WishItem{Id: "1", ItemType: "book"})
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestFileStore_GetItem(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}, Items: []domain.WishItem{{Id: "1", Name: "item1"}}},
	})

	got, err := fs.GetItem("pedro", "1")
	if err != nil {
		t.Fatalf("GetItem() error: %v", err)
	}
	if got.Name != "item1" {
		t.Fatalf("expected item1, got %+v", got)
	}

	if _, err := fs.GetItem("pedro", "missing"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for a missing item id, got %v", err)
	}
	if _, err := fs.GetItem("missing", "1"); !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound for a missing list, got %v", err)
	}
}

func TestFileStore_UpdateItem(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{
			List: domain.List{Slug: "pedro", Name: "Pedro"},
			Items: []domain.WishItem{
				{Id: "1", Name: "old", ItemType: "book"},
				{Id: "2", Name: "untouched", ItemType: "t-shirt"},
			},
		},
	})

	if err := fs.UpdateItem("pedro", domain.WishItem{Id: "1", Name: "new", ItemType: "book"}); err != nil {
		t.Fatalf("UpdateItem() error: %v", err)
	}

	got, err := fs.LoadList("pedro")
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

func TestFileStore_UpdateItem_NotFound(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{List: domain.List{Slug: "pedro", Name: "Pedro"}, Items: []domain.WishItem{{Id: "1"}}},
	})

	err := fs.UpdateItem("pedro", domain.WishItem{Id: "missing", ItemType: "book"})
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}

	err = fs.UpdateItem("missing", domain.WishItem{Id: "1", ItemType: "book"})
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestFileStore_DeleteItem(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{
			List:  domain.List{Slug: "pedro", Name: "Pedro"},
			Items: []domain.WishItem{{Id: "1"}, {Id: "2"}},
		},
	})

	if err := fs.DeleteItem("pedro", "1"); err != nil {
		t.Fatalf("DeleteItem() error: %v", err)
	}

	got, err := fs.LoadList("pedro")
	if err != nil {
		t.Fatalf("LoadList() error: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Id != "2" {
		t.Fatalf("expected only item 2 left, got %+v", got.Items)
	}

	if err := fs.DeleteItem("pedro", "1"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound deleting an already-removed item, got %v", err)
	}
	if err := fs.DeleteItem("missing", "2"); !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestFileStore_PurchaseItem(t *testing.T) {
	fs := newTestFileStore(t, []domain.ListWithItems{
		{
			List:  domain.List{Slug: "pedro", Name: "Pedro"},
			Items: []domain.WishItem{{Id: "1", WasPurchased: false}},
		},
	})

	got, err := fs.PurchaseItem("pedro", "1")
	if err != nil {
		t.Fatalf("PurchaseItem() error: %v", err)
	}
	if !got.WasPurchased {
		t.Fatalf("expected the returned item marked purchased, got %+v", got)
	}

	persisted, err := fs.LoadList("pedro")
	if err != nil {
		t.Fatalf("LoadList() error: %v", err)
	}
	if !persisted.Items[0].WasPurchased {
		t.Fatalf("expected the purchase to persist, got %+v", persisted.Items[0])
	}

	if _, err := fs.PurchaseItem("pedro", "missing"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
	if _, err := fs.PurchaseItem("missing", "1"); !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestFileStore_WriteLeavesOnlyTheWishlistFile(t *testing.T) {
	dir := t.TempDir()
	fs := &FileStore{path: filepath.Join(dir, "wishlist.json")}
	if err := fs.ReplaceAll([]domain.ListWithItems{{List: domain.List{Slug: "pedro", Name: "Pedro"}}}); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	if err := fs.CreateList(domain.List{Slug: "natal", Name: "Natal"}); err != nil {
		t.Fatalf("creating list: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "wishlist.json" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected only wishlist.json in the directory, found %v", names)
	}

	all, err := fs.LoadAll()
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 lists after the save, got %d", len(all))
	}
}
