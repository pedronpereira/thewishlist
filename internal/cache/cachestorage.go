package cache

import (
	"github.com/pedronpereira/thewishlist/internal/domain"
	"github.com/pedronpereira/thewishlist/internal/storage"
)

type InMemoryStorageCache struct {
	store storage.Store
}

func New() *InMemoryStorageCache {
	return &InMemoryStorageCache{}
}

// Load() domain.Wishlist
// SaveWishList(payload domain.Wishlist) error

func (mc *InMemoryStorageCache) Load() domain.Wishlist {
	return mc.store.Load()
}

func (mc *InMemoryStorageCache) SaveWishList(payload domain.Wishlist) error {
	return mc.store.SaveWishList(payload)
}
