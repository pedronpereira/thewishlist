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

// Searches all items for the purchased item and returns the updated item
func (w *Wishlist) ItemPurchased(id string) *WishItem {
	for i, wishitem := range w.Other {
		if wishitem.Id == id {
			wishitem.WasPurchased = !wishitem.WasPurchased
			w.Other[i] = wishitem
			return &wishitem
		}
	}

	for i, wishitem := range w.Books {
		if wishitem.Id == id {
			wishitem.WasPurchased = !wishitem.WasPurchased
			w.Books[i] = wishitem
			return &wishitem
		}
	}

	for i, wishitem := range w.Tshirts {
		if wishitem.Id == id {
			wishitem.WasPurchased = !wishitem.WasPurchased
			w.Tshirts[i] = wishitem
			return &wishitem
		}
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

	var collection *[]WishItem
	switch requestItem.ItemType {
	case "t-shirt":
		collection = &w.Tshirts
	case "book":
		collection = &w.Books
	default:
		collection = &w.Other
	}

	index := slices.IndexFunc(*collection, SearchByIndex(requestItem))
	if index == -1 {
		return "", fmt.Errorf("item not found")
	}

	(*collection)[index] = requestItem
	return requestItem.ItemType, nil
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

func (w *Wishlist) GetItem(itemType string, index int) WishItem {
	switch itemType {
	case "t-shirt":
		return w.Tshirts[index]
	case "book":
		return w.Books[index]
	default:
		return w.Other[index]
	}
}
