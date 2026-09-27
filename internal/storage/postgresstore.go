package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pedronpereira/thewishlist/internal/domain"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

const createTableSQL = `CREATE TABLE IF NOT EXISTS wish_items (
	id            TEXT PRIMARY KEY,
	name          TEXT NOT NULL DEFAULT '',
	title         TEXT NOT NULL DEFAULT '',
	description   TEXT NOT NULL DEFAULT '',
	item_type     TEXT NOT NULL DEFAULT '',
	shop_url      TEXT NOT NULL DEFAULT '',
	was_purchased BOOLEAN NOT NULL DEFAULT FALSE,
	img_source    TEXT NOT NULL DEFAULT ''
)`

func NewPostgresStore(ctx context.Context, connString string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("creating postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}

	if _, err := pool.Exec(ctx, createTableSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ensuring wish_items schema: %w", err)
	}

	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Load() (domain.Wishlist, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, title, description, item_type, shop_url, was_purchased, img_source
		 FROM wish_items ORDER BY id`)
	if err != nil {
		return domain.Wishlist{}, fmt.Errorf("loading wishlist from postgres: %w", err)
	}
	defer rows.Close()

	var items []domain.WishItem
	for rows.Next() {
		var item domain.WishItem
		if err := rows.Scan(&item.Id, &item.Name, &item.Title, &item.Description,
			&item.ItemType, &item.ShopUrl, &item.WasPurchased, &item.ImgSource); err != nil {
			return domain.Wishlist{}, fmt.Errorf("scanning wish item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.Wishlist{}, fmt.Errorf("reading wishlist rows: %w", err)
	}

	return domain.Wishlist{Items: items}, nil
}

func (s *PostgresStore) SaveWishList(payload domain.Wishlist) error {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "DELETE FROM wish_items"); err != nil {
		return fmt.Errorf("clearing wish items: %w", err)
	}

	batch := &pgx.Batch{}
	for _, item := range payload.Items {
		batch.Queue(
			`INSERT INTO wish_items (id, name, title, description, item_type, shop_url, was_purchased, img_source)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			item.Id, item.Name, item.Title, item.Description,
			item.ItemType, item.ShopUrl, item.WasPurchased, item.ImgSource,
		)
	}
	if batch.Len() > 0 {
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return fmt.Errorf("inserting wish items: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing wishlist save: %w", err)
	}
	return nil
}
