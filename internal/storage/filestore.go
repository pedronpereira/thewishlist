package storage

import (
	"encoding/json"
	"fmt"
	"os"

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

func (fs *FileStore) writeAll(payload fileFormat) error {
	buf, err := json.MarshalIndent(payload, "", "    ")
	if err != nil {
		return fmt.Errorf("marshaling wishlist file: %w", err)
	}

	if err := os.WriteFile(fs.path, buf, 0644); err != nil {
		return fmt.Errorf("writing wishlist file %q: %w", fs.path, err)
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
