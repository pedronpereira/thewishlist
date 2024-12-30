package storage

import (
	"github.com/pedronpereira/thewishlist/internal/domain"
)

type Store interface {
	Load() domain.Wishlist
	SaveWishList(payload domain.Wishlist) error
}

func NewFileStore(path string) *FileStore {
	return &FileStore{
		path: path,
	}
}

func NewCloudStore(uri string, dbName string, collection string) *MongoCloudStore {
	return &MongoCloudStore{
		uri:        uri,
		dbName:     dbName,
		collection: collection,
	}
}
