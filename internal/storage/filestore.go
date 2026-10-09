package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pedronpereira/thewishlist/internal/domain"
)

type FileStore struct {
	path string
}

// fileFormat is the on-disk shape of the JSON file: every list, each with
// its own items.
type fileFormat struct {
	Lists []domain.ListWithItems `json:"lists"`
}

func (fs *FileStore) readAll() (fileFormat, error) {
	data, err := os.ReadFile(fs.path)
	if err != nil {
		return fileFormat{}, fmt.Errorf("read wishlist file %q: %w", fs.path, err)
	}

	var payload fileFormat
	if err := json.Unmarshal(data, &payload); err != nil {
		return fileFormat{}, fmt.Errorf("parsing json file %q: %w", fs.path, err)
	}

	return payload, nil
}

// writeAll replaces the file atomically: the new contents go to a temp file
// in the same directory, are flushed to disk, and only then renamed over the
// real file. A crash mid-write leaves the previous file intact instead of a
// truncated one.
func (fs *FileStore) writeAll(payload fileFormat) error {
	buf, err := json.MarshalIndent(payload, "", "    ")
	if err != nil {
		return fmt.Errorf("marshaling wishlist file: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(fs.path), filepath.Base(fs.path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file for %q: %w", fs.path, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // only does anything if the rename below never happened

	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file %q: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("flushing temp file %q: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file %q: %w", tmpPath, err)
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return fmt.Errorf("setting permissions on %q: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, fs.path); err != nil {
		return fmt.Errorf("replacing wishlist file %q: %w", fs.path, err)
	}

	return nil
}

func (fs *FileStore) GetLists() ([]domain.List, error) {
	all, err := fs.readAll()
	if err != nil {
		return nil, err
	}

	lists := make([]domain.List, 0, len(all.Lists))
	for _, l := range all.Lists {
		list := l.List
		list.ItemCount = len(l.Items)
		lists = append(lists, list)
	}

	return lists, nil
}

func (fs *FileStore) GetList(slug string) (domain.List, error) {
	all, err := fs.readAll()
	if err != nil {
		return domain.List{}, err
	}

	for _, l := range all.Lists {
		if l.Slug == slug {
			list := l.List
			list.ItemCount = len(l.Items)
			return list, nil
		}
	}

	return domain.List{}, fmt.Errorf("list %q: %w", slug, ErrListNotFound)
}

func (fs *FileStore) LoadList(slug string) (domain.Wishlist, error) {
	all, err := fs.readAll()
	if err != nil {
		return domain.Wishlist{}, err
	}

	for _, l := range all.Lists {
		if l.Slug == slug {
			return domain.Wishlist{Items: l.Items}, nil
		}
	}

	return domain.Wishlist{}, fmt.Errorf("list %q: %w", slug, ErrListNotFound)
}

func (fs *FileStore) SaveList(slug string, w domain.Wishlist) error {
	all, err := fs.readAll()
	if err != nil {
		return err
	}

	for i, l := range all.Lists {
		if l.Slug == slug {
			all.Lists[i].Items = w.Items
			return fs.writeAll(all)
		}
	}

	return fmt.Errorf("list %q: %w", slug, ErrListNotFound)
}

// AddItem inserts a new item into slug's list, or replaces an existing item
// with the same id in place (upsert), matching the create handler's
// semantics without rewriting every other item in the list.
func (fs *FileStore) AddItem(slug string, item domain.WishItem) error {
	all, err := fs.readAll()
	if err != nil {
		return err
	}

	for i, l := range all.Lists {
		if l.Slug != slug {
			continue
		}
		wishlist := domain.Wishlist{Items: l.Items}
		if wishlist.IndexOf(item) == -1 {
			wishlist.AddItem(item)
		} else if _, err := wishlist.UpdateItem(item); err != nil {
			return err
		}
		all.Lists[i].Items = wishlist.Items
		return fs.writeAll(all)
	}

	return fmt.Errorf("list %q: %w", slug, ErrListNotFound)
}

func (fs *FileStore) GetItem(slug, id string) (domain.WishItem, error) {
	all, err := fs.readAll()
	if err != nil {
		return domain.WishItem{}, err
	}

	for _, l := range all.Lists {
		if l.Slug != slug {
			continue
		}
		wishlist := domain.Wishlist{Items: l.Items}
		if index := wishlist.IndexOf(domain.WishItem{Id: id}); index != -1 {
			return wishlist.Items[index], nil
		}
		return domain.WishItem{}, fmt.Errorf("item %q: %w", id, ErrItemNotFound)
	}

	return domain.WishItem{}, fmt.Errorf("list %q: %w", slug, ErrListNotFound)
}

func (fs *FileStore) UpdateItem(slug string, item domain.WishItem) error {
	all, err := fs.readAll()
	if err != nil {
		return err
	}

	for i, l := range all.Lists {
		if l.Slug != slug {
			continue
		}
		wishlist := domain.Wishlist{Items: l.Items}
		if _, err := wishlist.UpdateItem(item); err != nil {
			return fmt.Errorf("item %q: %w", item.Id, ErrItemNotFound)
		}
		all.Lists[i].Items = wishlist.Items
		return fs.writeAll(all)
	}

	return fmt.Errorf("list %q: %w", slug, ErrListNotFound)
}

func (fs *FileStore) DeleteItem(slug, id string) error {
	all, err := fs.readAll()
	if err != nil {
		return err
	}

	for i, l := range all.Lists {
		if l.Slug != slug {
			continue
		}
		wishlist := domain.Wishlist{Items: l.Items}
		if !wishlist.RemoveItem(id) {
			return fmt.Errorf("item %q: %w", id, ErrItemNotFound)
		}
		all.Lists[i].Items = wishlist.Items
		return fs.writeAll(all)
	}

	return fmt.Errorf("list %q: %w", slug, ErrListNotFound)
}

func (fs *FileStore) PurchaseItem(slug, id string) (domain.WishItem, error) {
	all, err := fs.readAll()
	if err != nil {
		return domain.WishItem{}, err
	}

	for i, l := range all.Lists {
		if l.Slug != slug {
			continue
		}
		wishlist := domain.Wishlist{Items: l.Items}
		item := wishlist.ItemPurchased(id)
		if item == nil {
			return domain.WishItem{}, fmt.Errorf("item %q: %w", id, ErrItemNotFound)
		}
		all.Lists[i].Items = wishlist.Items
		if err := fs.writeAll(all); err != nil {
			return domain.WishItem{}, err
		}
		return *item, nil
	}

	return domain.WishItem{}, fmt.Errorf("list %q: %w", slug, ErrListNotFound)
}

func (fs *FileStore) LoadAll() ([]domain.ListWithItems, error) {
	all, err := fs.readAll()
	if err != nil {
		return nil, err
	}

	return all.Lists, nil
}

func (fs *FileStore) ReplaceAll(lists []domain.ListWithItems) error {
	return fs.writeAll(fileFormat{Lists: lists})
}

func (fs *FileStore) CreateList(list domain.List) error {
	all, err := fs.readAll()
	if err != nil {
		return err
	}

	for _, l := range all.Lists {
		if l.Slug == list.Slug {
			return fmt.Errorf("list %q: %w", list.Slug, ErrListExists)
		}
	}

	all.Lists = append(all.Lists, domain.ListWithItems{List: list, Items: []domain.WishItem{}})
	return fs.writeAll(all)
}
