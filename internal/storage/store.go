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
	// an existing list's items, it does not create new lists. It replaces
	// every item in the list in one shot; the single-item methods below are
	// cheaper for mutating just one item, which is what every handler now
	// does, so nothing currently calls SaveList — kept as a general
	// bulk-replace primitive.
	SaveList(slug string, w domain.Wishlist) error

	// AddItem inserts a new item into the named list, or replaces an
	// existing item with the same id in place (upsert) — matching the
	// create handler's "create or overwrite" semantics in one targeted
	// write, instead of loading, deleting, and reinserting every item in
	// the list via SaveList. Returns ErrListNotFound for an unknown slug.
	AddItem(slug string, item domain.WishItem) error
	// GetItem returns one item from the named list. Returns ErrListNotFound
	// for an unknown slug, ErrItemNotFound if the list exists but has no
	// item with that id.
	GetItem(slug, id string) (domain.WishItem, error)
	// UpdateItem replaces an existing item's fields in place. Returns
	// ErrListNotFound for an unknown slug, ErrItemNotFound if the list
	// exists but has no item with that id.
	UpdateItem(slug string, item domain.WishItem) error
	// DeleteItem removes one item from the named list. Returns
	// ErrListNotFound for an unknown slug, ErrItemNotFound if the list
	// exists but has no item with that id.
	DeleteItem(slug, id string) error
	// PurchaseItem marks one item as purchased and returns its resulting
	// state. Returns ErrListNotFound for an unknown slug, ErrItemNotFound
	// if the list exists but has no item with that id.
	PurchaseItem(slug, id string) (domain.WishItem, error)

	// LoadAll and ReplaceAll operate across every list at once, for the
	// bulk GET/POST /wishlist export/import endpoints.
	LoadAll() ([]domain.ListWithItems, error)
	ReplaceAll(lists []domain.ListWithItems) error

	// CreateList adds an empty list. It returns ErrListExists if the slug is
	// already taken, so the caller can report a conflict rather than a
	// generic failure.
	CreateList(list domain.List) error
}

var (
	ErrListNotFound = errors.New("list not found")
	ErrListExists   = errors.New("list already exists")
	ErrItemNotFound = errors.New("item not found")
)

func NewFileStore(path string) *FileStore {
	return &FileStore{
		path: path,
	}
}
