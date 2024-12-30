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

func (fs *FileStore) Load() domain.Wishlist {
	fmt.Println("Loading data from file")

	data, err := os.ReadFile(fs.path)
	if err != nil {
		fmt.Printf("ERROR %s: %v", "Reading file", err)
	}

	var payload domain.Wishlist
	err = json.Unmarshal(data, &payload)
	if err != nil {
		fmt.Printf("ERROR %s: %v", "Parsing json", err)
	}

	return payload
}

func (fs *FileStore) SaveWishList(payload domain.Wishlist) error {
	buf, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ERROR trying to marshal wishlist %s", err)
	}

	err = os.WriteFile(fs.path, buf, 0644)
	if err != nil {
		return fmt.Errorf("ERROR trying to update file %s", err)
	}

	return nil
}
