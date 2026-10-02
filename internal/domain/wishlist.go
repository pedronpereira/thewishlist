package domain

import (
	"fmt"
	"slices"
)

type Wishlist struct {
	Items []WishItem `json:"items"`
}

// ItemPurchased finds the item by id, marks it as purchased, and returns it.
func (w *Wishlist) ItemPurchased(id string) *WishItem {
	i := slices.IndexFunc(w.Items, func(item WishItem) bool { return item.Id == id })
	if i == -1 {
		return nil
	}

	w.Items[i].WasPurchased = true
	return &w.Items[i]
}

func (w *Wishlist) UpdateItem(requestItem WishItem) (string, error) {
	if requestItem.Id == "" {
		return "", fmt.Errorf("item has no id")
	}

	if requestItem.ItemType == "" {
		return "", fmt.Errorf("item has no type")
	}

	index := slices.IndexFunc(w.Items, SearchByIndex(requestItem))
	if index == -1 {
		return "", fmt.Errorf("item not found")
	}

	w.Items[index] = requestItem
	return requestItem.ItemType, nil
}

func (w *Wishlist) IndexOf(item WishItem) int {
	return slices.IndexFunc(w.Items, SearchByIndex(item))
}

func SearchByIndex(item WishItem) func(WishItem) bool {
	return func(i WishItem) bool { return i.Id == item.Id }
}

func (w *Wishlist) AddItem(item WishItem) {
	w.Items = append(w.Items, item)
}

// RemoveItem removes the item with the given id, if present, and reports
// whether it was found.
func (w *Wishlist) RemoveItem(id string) bool {
	i := slices.IndexFunc(w.Items, func(item WishItem) bool { return item.Id == id })
	if i == -1 {
		return false
	}

	w.Items = slices.Delete(w.Items, i, i+1)
	return true
}
