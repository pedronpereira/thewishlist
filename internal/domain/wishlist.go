package domain

import (
	"fmt"
	"slices"
)

type Wishlist struct {
	Id      string `json:"_id"`
	Count   int
	Tshirts []WishItem
	Books   []WishItem
	Other   []WishItem
}

// Clone returns a deep copy of the wishlist. Because WishItem holds no
// reference types of its own, cloning the three slices is enough to ensure
// mutations to the clone (including in-place replacement of an item) never
// alias w's underlying arrays.
func (w Wishlist) Clone() Wishlist {
	return Wishlist{
		Id:      w.Id,
		Count:   w.Count,
		Tshirts: slices.Clone(w.Tshirts),
		Books:   slices.Clone(w.Books),
		Other:   slices.Clone(w.Other),
	}
}

// Searches all items for the purchased item and returns the updated item
func (w *Wishlist) ItemPurchased(id string) *WishItem {
	i := slices.IndexFunc(w.Tshirts, func(item WishItem) bool { return item.Id == id })
	if i != -1 {
		w.Tshirts[i].WasPurchased = true
		return &w.Tshirts[i]
	}

	i = slices.IndexFunc(w.Other, func(item WishItem) bool { return item.Id == id })
	if i != -1 {
		w.Other[i].WasPurchased = true
		return &w.Other[i]
	}

	i = slices.IndexFunc(w.Books, func(item WishItem) bool { return item.Id == id })
	if i != -1 {
		w.Books[i].WasPurchased = true
		return &w.Books[i]
	}

	return nil
}

func (w *Wishlist) UpdateItem(requestItem WishItem) (string, error) {
	if requestItem.Id == "" {
		return "", fmt.Errorf("item has no id")
	}

	if requestItem.ItemType == "" {
		return "", fmt.Errorf("item has no type")
	}

	currentCollection, index := w.findByID(requestItem.Id)
	if index == -1 {
		return "", fmt.Errorf("item not found")
	}

	targetCollection := w.collectionFor(requestItem.ItemType)

	if currentCollection == targetCollection {
		(*currentCollection)[index] = requestItem
		return requestItem.ItemType, nil
	}

	// ItemType changed: move the item into its new collection instead of
	// leaving a stale copy behind in the old one.
	*currentCollection = slices.Delete(*currentCollection, index, index+1)
	*targetCollection = append(*targetCollection, requestItem)
	return requestItem.ItemType, nil
}

func (w *Wishlist) collectionFor(itemType string) *[]WishItem {
	switch itemType {
	case "t-shirt":
		return &w.Tshirts
	case "book":
		return &w.Books
	default:
		return &w.Other
	}
}

// findByID searches every collection for an item by id, regardless of its
// ItemType, and returns a pointer to the collection it was found in along
// with its index. It returns (nil, -1) if no item matches.
func (w *Wishlist) findByID(id string) (*[]WishItem, int) {
	if i := slices.IndexFunc(w.Tshirts, func(item WishItem) bool { return item.Id == id }); i != -1 {
		return &w.Tshirts, i
	}
	if i := slices.IndexFunc(w.Books, func(item WishItem) bool { return item.Id == id }); i != -1 {
		return &w.Books, i
	}
	if i := slices.IndexFunc(w.Other, func(item WishItem) bool { return item.Id == id }); i != -1 {
		return &w.Other, i
	}
	return nil, -1
}

func (w *Wishlist) IndexOf(item WishItem) int {
	index := -1
	switch item.ItemType {
	case "t-shirt":
		index = slices.IndexFunc(w.Tshirts, SearchByIndex(item))
	case "book":
		index = slices.IndexFunc(w.Books, SearchByIndex(item))
	default:
		index = slices.IndexFunc(w.Other, SearchByIndex(item))
	}

	return index
}

func SearchByIndex(item WishItem) func(WishItem) bool {
	return func(i WishItem) bool { return i.Id == item.Id }
}

func (w *Wishlist) AddItem(item WishItem) {
	switch item.ItemType {
	case "t-shirt":
		w.Tshirts = append(w.Tshirts, item)
	case "book":
		w.Books = append(w.Books, item)
	default:
		w.Other = append(w.Other, item)
	}
}
