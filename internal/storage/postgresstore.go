package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pedronpereira/thewishlist/internal/domain"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

const (
	createWishItemsTableSQL = `CREATE TABLE IF NOT EXISTS wish_items (
	id            TEXT PRIMARY KEY,
	name          TEXT NOT NULL DEFAULT '',
	title         TEXT NOT NULL DEFAULT '',
	description   TEXT NOT NULL DEFAULT '',
	item_type     TEXT NOT NULL DEFAULT '',
	shop_url      TEXT NOT NULL DEFAULT '',
	was_purchased BOOLEAN NOT NULL DEFAULT FALSE,
	img_source    TEXT NOT NULL DEFAULT ''
)`

	createListsTableSQL = `CREATE TABLE IF NOT EXISTS lists (
	id                 BIGSERIAL PRIMARY KEY,
	slug               TEXT UNIQUE NOT NULL,
	name               TEXT NOT NULL,
	icon               TEXT NOT NULL DEFAULT '',
	is_admin_recipient BOOLEAN NOT NULL DEFAULT FALSE,
	is_default         BOOLEAN NOT NULL DEFAULT FALSE
)`

	// Seeds the default list so existing (pre-multi-list) deployments have
	// somewhere for their already-live items to attach to below.
	seedDefaultListSQL = `INSERT INTO lists (slug, name, icon, is_admin_recipient, is_default)
	VALUES ('pedro', 'Pedro', '🎁', TRUE, TRUE)
	ON CONFLICT (slug) DO NOTHING`

	// Backfills every existing wish_items row onto the seeded default list
	// via the column default — this is what migrates already-live data.
	addListSlugColumnSQL = `ALTER TABLE wish_items
	ADD COLUMN IF NOT EXISTS list_slug TEXT NOT NULL DEFAULT 'pedro' REFERENCES lists(slug)`

	createListSlugIndexSQL = `CREATE INDEX IF NOT EXISTS wish_items_list_slug_idx ON wish_items(list_slug)`
)

func NewPostgresStore(ctx context.Context, connString string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("creating postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}

	// Order matters: wish_items must exist before it can be ALTERed, and
	// lists (plus the seed row) must exist before wish_items' new column
	// can reference it.
	migrations := []string{
		createWishItemsTableSQL,
		createListsTableSQL,
		seedDefaultListSQL,
		addListSlugColumnSQL,
		createListSlugIndexSQL,
	}
	for _, stmt := range migrations {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			pool.Close()
			return nil, fmt.Errorf("running schema migration: %w", err)
		}
	}

	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) GetLists() ([]domain.List, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		SELECT lists.slug, lists.name, lists.icon, lists.is_admin_recipient, lists.is_default,
		       COUNT(wish_items.id)
		FROM lists
		LEFT JOIN wish_items ON wish_items.list_slug = lists.slug
		GROUP BY lists.id
		ORDER BY lists.id`)
	if err != nil {
		return nil, fmt.Errorf("loading lists from postgres: %w", err)
	}
	defer rows.Close()

	var lists []domain.List
	for rows.Next() {
		var l domain.List
		if err := rows.Scan(&l.Slug, &l.Name, &l.Icon, &l.IsAdminRecipient, &l.IsDefault, &l.ItemCount); err != nil {
			return nil, fmt.Errorf("scanning list: %w", err)
		}
		lists = append(lists, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading list rows: %w", err)
	}

	return lists, nil
}

func (s *PostgresStore) listExists(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM lists WHERE slug = $1)`, slug).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking list %q exists: %w", slug, err)
	}
	return exists, nil
}

func (s *PostgresStore) GetList(slug string) (domain.List, error) {
	ctx := context.Background()

	var l domain.List
	err := s.pool.QueryRow(ctx, `
		SELECT lists.slug, lists.name, lists.icon, lists.is_admin_recipient, lists.is_default,
		       COUNT(wish_items.id)
		FROM lists
		LEFT JOIN wish_items ON wish_items.list_slug = lists.slug
		WHERE lists.slug = $1
		GROUP BY lists.id`, slug,
	).Scan(&l.Slug, &l.Name, &l.Icon, &l.IsAdminRecipient, &l.IsDefault, &l.ItemCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.List{}, fmt.Errorf("list %q: %w", slug, ErrListNotFound)
	}
	if err != nil {
		return domain.List{}, fmt.Errorf("loading list %q from postgres: %w", slug, err)
	}

	return l, nil
}

func (s *PostgresStore) LoadList(slug string) (domain.Wishlist, error) {
	ctx := context.Background()

	exists, err := s.listExists(ctx, slug)
	if err != nil {
		return domain.Wishlist{}, err
	}
	if !exists {
		return domain.Wishlist{}, fmt.Errorf("list %q: %w", slug, ErrListNotFound)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, name, title, description, item_type, shop_url, was_purchased, img_source
		 FROM wish_items WHERE list_slug = $1 ORDER BY id`, slug)
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

func (s *PostgresStore) SaveList(slug string, w domain.Wishlist) error {
	ctx := context.Background()

	exists, err := s.listExists(ctx, slug)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("list %q: %w", slug, ErrListNotFound)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "DELETE FROM wish_items WHERE list_slug = $1", slug); err != nil {
		return fmt.Errorf("clearing wish items for list %q: %w", slug, err)
	}

	batch := &pgx.Batch{}
	for _, item := range w.Items {
		batch.Queue(
			`INSERT INTO wish_items (id, name, title, description, item_type, shop_url, was_purchased, img_source, list_slug)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			item.Id, item.Name, item.Title, item.Description,
			item.ItemType, item.ShopUrl, item.WasPurchased, item.ImgSource, slug,
		)
	}
	if batch.Len() > 0 {
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return fmt.Errorf("inserting wish items for list %q: %w", slug, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing list %q save: %w", slug, err)
	}
	return nil
}

// LoadAll loads every list's metadata then its items one list at a time.
// N+1 queries, deliberately not optimized further: this only backs the
// rarely-used full-export debug endpoint, and the list count here is tiny.
func (s *PostgresStore) LoadAll() ([]domain.ListWithItems, error) {
	lists, err := s.GetLists()
	if err != nil {
		return nil, err
	}

	result := make([]domain.ListWithItems, 0, len(lists))
	for _, l := range lists {
		wishlist, err := s.LoadList(l.Slug)
		if err != nil {
			return nil, err
		}
		result = append(result, domain.ListWithItems{List: l, Items: wishlist.Items})
	}

	return result, nil
}

func (s *PostgresStore) CreateList(list domain.List) error {
	ctx := context.Background()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO lists (slug, name, icon, is_admin_recipient, is_default) VALUES ($1, $2, $3, $4, FALSE)`,
		list.Slug, list.Name, list.Icon, list.IsAdminRecipient,
	)

	// 23505 is Postgres's unique_violation: the slug column is UNIQUE.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("list %q: %w", list.Slug, ErrListExists)
	}
	if err != nil {
		return fmt.Errorf("inserting list %q: %w", list.Slug, err)
	}
	return nil
}

func (s *PostgresStore) ReplaceAll(lists []domain.ListWithItems) error {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// wish_items first: it has an FK to lists, so it must be cleared before
	// lists can be cleared and safely repopulated with the new set.
	if _, err := tx.Exec(ctx, "DELETE FROM wish_items"); err != nil {
		return fmt.Errorf("clearing wish items: %w", err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM lists"); err != nil {
		return fmt.Errorf("clearing lists: %w", err)
	}

	for _, l := range lists {
		if _, err := tx.Exec(ctx,
			`INSERT INTO lists (slug, name, icon, is_admin_recipient, is_default) VALUES ($1, $2, $3, $4, $5)`,
			l.Slug, l.Name, l.Icon, l.IsAdminRecipient, l.IsDefault,
		); err != nil {
			return fmt.Errorf("inserting list %q: %w", l.Slug, err)
		}
	}

	batch := &pgx.Batch{}
	for _, l := range lists {
		for _, item := range l.Items {
			batch.Queue(
				`INSERT INTO wish_items (id, name, title, description, item_type, shop_url, was_purchased, img_source, list_slug)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				item.Id, item.Name, item.Title, item.Description,
				item.ItemType, item.ShopUrl, item.WasPurchased, item.ImgSource, l.Slug,
			)
		}
	}
	if batch.Len() > 0 {
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return fmt.Errorf("inserting wish items: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing replace-all: %w", err)
	}
	return nil
}
