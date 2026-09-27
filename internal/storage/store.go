package storage

import (
	"github.com/pedronpereira/thewishlist/internal/domain"
)

type Store interface {
	Load() (domain.Wishlist, error)
	SaveWishList(payload domain.Wishlist) error
}

func NewFileStore(path string) *FileStore {
	return &FileStore{
		path: path,
	}
}
