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

	if requestItem.ItemType == "t-shirt" {
		for i, wishitem := range w.Tshirts {
			if wishitem.Id == requestItem.Id {
				w.Tshirts[i] = requestItem
				return requestItem.ItemType, nil
			}
		}
	}

	if requestItem.ItemType == "book" {
		for i, wishitem := range w.Books {
			if wishitem.Id == requestItem.Id {
				w.Books[i] = requestItem
				return requestItem.ItemType, nil
			}
		}
	}

	if requestItem.ItemType == "mouse-mat" {
		for i, wishitem := range w.Other {
			if wishitem.Id == requestItem.Id {
				w.Other[i] = requestItem
				return requestItem.ItemType, nil
			}
		}
	}

	return "", fmt.Errorf("item not found")
}

func (w *Wishlist) IndexOf(item WishItem) int {
	index := -1
	switch item.ItemType {
	case "t-shirt":
		index = slices.IndexFunc(w.Tshirts, func(i WishItem) bool {
			return i.Id == item.Id
		})
	case "book":
		index = slices.IndexFunc(w.Books, func(i WishItem) bool {
			return i.Id == item.Id
		})
	default:
		index = slices.IndexFunc(w.Other, func(i WishItem) bool {
			return i.Id == item.Id
		})
	}

	return index
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
