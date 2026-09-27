package domain

import "testing"

func TestClone(t *testing.T) {
	w := Wishlist{
		Id:      "1",
		Count:   2,
		Tshirts: []WishItem{{Id: "t1", Name: "old"}},
		Books:   []WishItem{{Id: "b1"}},
		Other:   []WishItem{{Id: "o1"}},
	}

	clone := w.Clone()

	// Mutate the clone in ways that would alias w's backing arrays if
	// Clone were a shallow copy: an in-place field change and an append.
	clone.Tshirts[0].Name = "new"
	clone.AddItem(WishItem{Id: "t2", ItemType: "t-shirt"})

	if w.Tshirts[0].Name != "old" {
		t.Fatalf("expected original Tshirts[0].Name unaffected, got %q", w.Tshirts[0].Name)
	}
	if len(w.Tshirts) != 1 {
		t.Fatalf("expected original Tshirts length unaffected, got %d", len(w.Tshirts))
	}
	if len(clone.Tshirts) != 2 {
		t.Fatalf("expected clone Tshirts length to grow independently, got %d", len(clone.Tshirts))
	}
}

func TestAddItem(t *testing.T) {
	tests := []struct {
		name     string
		itemType string
	}{
		{name: "t-shirt goes to Tshirts", itemType: "t-shirt"},
		{name: "book goes to Books", itemType: "book"},
		{name: "unrecognized type goes to Other", itemType: "poster"},
		{name: "empty type goes to Other", itemType: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Wishlist{}
			item := WishItem{Id: "1", ItemType: tt.itemType}
			w.AddItem(item)

			switch tt.itemType {
			case "t-shirt":
				if len(w.Tshirts) != 1 || w.Tshirts[0].Id != "1" {
					t.Fatalf("expected item in Tshirts, got %+v", w)
				}
			case "book":
				if len(w.Books) != 1 || w.Books[0].Id != "1" {
					t.Fatalf("expected item in Books, got %+v", w)
				}
			default:
				if len(w.Other) != 1 || w.Other[0].Id != "1" {
					t.Fatalf("expected item in Other, got %+v", w)
				}
			}
		})
	}
}

func TestAddItem_AppendsToExisting(t *testing.T) {
	w := Wishlist{Tshirts: []WishItem{{Id: "1", ItemType: "t-shirt"}}}
	w.AddItem(WishItem{Id: "2", ItemType: "t-shirt"})

	if len(w.Tshirts) != 2 {
		t.Fatalf("expected 2 t-shirts, got %d", len(w.Tshirts))
	}
	if w.Tshirts[0].Id != "1" || w.Tshirts[1].Id != "2" {
		t.Fatalf("expected existing item preserved and new item appended, got %+v", w.Tshirts)
	}
}

func TestIndexOf(t *testing.T) {
	w := Wishlist{
		Tshirts: []WishItem{{Id: "t1", ItemType: "t-shirt"}},
		Books:   []WishItem{{Id: "b1", ItemType: "book"}},
		Other:   []WishItem{{Id: "o1", ItemType: "other"}},
	}

	tests := []struct {
		name string
		item WishItem
		want int
	}{
		{name: "found in Tshirts", item: WishItem{Id: "t1", ItemType: "t-shirt"}, want: 0},
		{name: "found in Books", item: WishItem{Id: "b1", ItemType: "book"}, want: 0},
		{name: "found in Other", item: WishItem{Id: "o1", ItemType: "other"}, want: 0},
		{name: "missing id in its collection", item: WishItem{Id: "nope", ItemType: "t-shirt"}, want: -1},
		{name: "id exists but under a different type", item: WishItem{Id: "t1", ItemType: "book"}, want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.IndexOf(tt.item)
			if got != tt.want {
				t.Fatalf("IndexOf() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestUpdateItem_Success(t *testing.T) {
	tests := []struct {
		name     string
		itemType string
	}{
		{name: "updates existing t-shirt", itemType: "t-shirt"},
		{name: "updates existing book", itemType: "book"},
		{name: "updates existing other item", itemType: "poster"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Wishlist{}
			w.AddItem(WishItem{Id: "1", ItemType: tt.itemType, Name: "old"})

			updated := WishItem{Id: "1", ItemType: tt.itemType, Name: "new"}
			gotType, err := w.UpdateItem(updated)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotType != tt.itemType {
				t.Fatalf("UpdateItem() type = %q, want %q", gotType, tt.itemType)
			}

			collection, index := w.findByID(updated.Id)
			if index == -1 {
				t.Fatalf("expected updated item to be findable")
			}
			if (*collection)[index].Name != "new" {
				t.Fatalf("expected item to be replaced with new value")
			}
		})
	}
}

func TestUpdateItem_Errors(t *testing.T) {
	tests := []struct {
		name    string
		item    WishItem
		wantErr string
	}{
		{name: "empty id", item: WishItem{Id: "", ItemType: "t-shirt"}, wantErr: "item has no id"},
		{name: "empty type", item: WishItem{Id: "1", ItemType: ""}, wantErr: "item has no type"},
		{name: "item not found in any collection", item: WishItem{Id: "missing", ItemType: "t-shirt"}, wantErr: "item not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Wishlist{}
			_, err := w.UpdateItem(tt.item)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("UpdateItem() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// Changing an item's ItemType on update moves it into the new collection
// rather than being treated as "not found" or leaving a stale copy behind.
func TestUpdateItem_MovesItemBetweenCollections(t *testing.T) {
	w := Wishlist{}
	w.AddItem(WishItem{Id: "1", ItemType: "t-shirt", Name: "old"})

	gotType, err := w.UpdateItem(WishItem{Id: "1", ItemType: "book", Name: "new"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotType != "book" {
		t.Fatalf("UpdateItem() type = %q, want %q", gotType, "book")
	}

	if len(w.Tshirts) != 0 {
		t.Fatalf("expected item removed from Tshirts, got %+v", w.Tshirts)
	}
	if len(w.Books) != 1 || w.Books[0].Name != "new" {
		t.Fatalf("expected item moved into Books with updated fields, got %+v", w.Books)
	}
}

func TestItemPurchased(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "found in Tshirts", id: "t1"},
		{name: "found in Other", id: "o1"},
		{name: "found in Books", id: "b1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Wishlist{
				Tshirts: []WishItem{{Id: "t1", ItemType: "t-shirt"}},
				Other:   []WishItem{{Id: "o1", ItemType: "other"}},
				Books:   []WishItem{{Id: "b1", ItemType: "book"}},
			}

			got := w.ItemPurchased(tt.id)
			if got == nil {
				t.Fatalf("expected item, got nil")
			}
			if got.Id != tt.id {
				t.Fatalf("ItemPurchased() returned id %q, want %q", got.Id, tt.id)
			}
			if !got.WasPurchased {
				t.Fatalf("expected WasPurchased to be true")
			}
		})
	}
}

func TestItemPurchased_NotFound(t *testing.T) {
	w := Wishlist{Tshirts: []WishItem{{Id: "t1", ItemType: "t-shirt"}}}

	got := w.ItemPurchased("missing")
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

// ItemPurchased checks Tshirts, then Other, then Books. This test pins that
// precedence for the (abnormal) case of a duplicate id across collections.
func TestItemPurchased_ChecksTshirtsBeforeOtherAndBooks(t *testing.T) {
	w := Wishlist{
		Tshirts: []WishItem{{Id: "dup", ItemType: "t-shirt"}},
		Other:   []WishItem{{Id: "dup", ItemType: "other"}},
		Books:   []WishItem{{Id: "dup", ItemType: "book"}},
	}

	got := w.ItemPurchased("dup")
	if got == nil || got.ItemType != "t-shirt" {
		t.Fatalf("expected match from Tshirts collection first, got %+v", got)
	}
	if w.Other[0].WasPurchased || w.Books[0].WasPurchased {
		t.Fatalf("expected only the Tshirts entry to be marked purchased")
	}
}

func TestSearchByIndex(t *testing.T) {
	predicate := SearchByIndex(WishItem{Id: "match"})

	if !predicate(WishItem{Id: "match", Name: "different name"}) {
		t.Fatalf("expected predicate to match on Id regardless of other fields")
	}
	if predicate(WishItem{Id: "no-match"}) {
		t.Fatalf("expected predicate to reject a different Id")
	}
}
