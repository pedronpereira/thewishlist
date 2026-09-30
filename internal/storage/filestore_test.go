package storage

import (
	"errors"
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
