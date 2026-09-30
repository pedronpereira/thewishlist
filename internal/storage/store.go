package storage

import (
	"errors"

	"github.com/pedronpereira/thewishlist/internal/domain"
)

type Store interface {
	// GetLists returns every list's metadata, including its item count, for
	// the tab bar and for resolving "/"'s default list.
	GetLists() ([]domain.List, error)
	// GetList returns one list's own metadata (no item count), for callers
	// that only need to know about a single already-known slug — cheaper
	// than GetLists for that case, and returns ErrListNotFound explicitly
	// rather than requiring the caller to search a list they fetched
	// separately (which could theoretically race with a concurrent delete).
	GetList(slug string) (domain.List, error)
	// LoadList returns ErrListNotFound (checkable via errors.Is) if slug
	// doesn't name an existing list.
	LoadList(slug string) (domain.Wishlist, error)
	// SaveList also returns ErrListNotFound for an unknown slug — it mutates
	// an existing list's items, it does not create new lists.
	SaveList(slug string, w domain.Wishlist) error

	// LoadAll and ReplaceAll operate across every list at once, for the
	// bulk GET/POST /wishlist export/import endpoints. ReplaceAll is also
	// the only way to introduce a brand-new list until list-creation UI
	// (Phase 3) exists.
	LoadAll() ([]domain.ListWithItems, error)
	ReplaceAll(lists []domain.ListWithItems) error
}

var ErrListNotFound = errors.New("list not found")

func NewFileStore(path string) *FileStore {
	return &FileStore{
		path: path,
	}
}
