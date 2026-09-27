package domain

import "testing"

func TestAddItem(t *testing.T) {
	w := Wishlist{}
	item := WishItem{Id: "1", ItemType: "t-shirt"}
	w.AddItem(item)

	if len(w.Items) != 1 || w.Items[0].Id != "1" {
		t.Fatalf("expected item appended to Items, got %+v", w.Items)
	}
}

func TestAddItem_AppendsToExisting(t *testing.T) {
	w := Wishlist{Items: []WishItem{{Id: "1", ItemType: "t-shirt"}}}
	w.AddItem(WishItem{Id: "2", ItemType: "book"})

	if len(w.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(w.Items))
	}
	if w.Items[0].Id != "1" || w.Items[1].Id != "2" {
		t.Fatalf("expected existing item preserved and new item appended, got %+v", w.Items)
	}
}

func TestIndexOf(t *testing.T) {
	w := Wishlist{
		Items: []WishItem{
			{Id: "t1", ItemType: "t-shirt"},
			{Id: "b1", ItemType: "book"},
			{Id: "o1", ItemType: "other"},
		},
	}

	tests := []struct {
		name string
		item WishItem
		want int
	}{
		{name: "found by id", item: WishItem{Id: "b1", ItemType: "book"}, want: 1},
		{name: "missing id", item: WishItem{Id: "nope", ItemType: "t-shirt"}, want: -1},
		// Flattening removes the old type-scoped lookup: a correct id is now
		// found regardless of the ItemType hint, fixing a latent bug where a
		// mismatched type on an otherwise-correct id used to report -1.
		{name: "found even with a mismatched type hint", item: WishItem{Id: "t1", ItemType: "book"}, want: 0},
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
	w := Wishlist{}
	w.AddItem(WishItem{Id: "1", ItemType: "t-shirt", Name: "old"})

	updated := WishItem{Id: "1", ItemType: "t-shirt", Name: "new"}
	gotType, err := w.UpdateItem(updated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotType != "t-shirt" {
		t.Fatalf("UpdateItem() type = %q, want %q", gotType, "t-shirt")
	}

	index := w.IndexOf(updated)
	if index == -1 {
		t.Fatalf("expected updated item to be findable")
	}
	if w.Items[index].Name != "new" {
		t.Fatalf("expected item to be replaced with new value")
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
		{name: "item not found", item: WishItem{Id: "missing", ItemType: "t-shirt"}, wantErr: "item not found"},
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

func TestItemPurchased(t *testing.T) {
	w := Wishlist{
		Items: []WishItem{
			{Id: "t1", ItemType: "t-shirt"},
			{Id: "o1", ItemType: "other"},
			{Id: "b1", ItemType: "book"},
		},
	}

	got := w.ItemPurchased("b1")
	if got == nil {
		t.Fatalf("expected item, got nil")
	}
	if got.Id != "b1" {
		t.Fatalf("ItemPurchased() returned id %q, want %q", got.Id, "b1")
	}
	if !got.WasPurchased {
		t.Fatalf("expected WasPurchased to be true")
	}
	if !w.Items[2].WasPurchased {
		t.Fatalf("expected the underlying item in Items to be marked purchased")
	}
}

func TestItemPurchased_NotFound(t *testing.T) {
	w := Wishlist{Items: []WishItem{{Id: "t1", ItemType: "t-shirt"}}}

	got := w.ItemPurchased("missing")
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
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
