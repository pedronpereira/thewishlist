# The Wishlist: Design

## Overview

The Wishlist is a small Go web application for publishing a personal list of
gift ideas. Visitors can browse the list, open links to stores, and mark an item
as purchased so that other visitors know it has already been bought.

The application uses:

- Go 1.23
- Echo for HTTP routing and middleware
- Go HTML templates for server-side rendering
- HTMX and Idiomorph for partial page updates
- JSON-file storage for local use
- MongoDB as an optional cloud store

## Architecture

The application is organized into a small set of layers:

```text
Browser
  |
  | HTML and HTMX requests
  v
Echo HTTP handlers
  |
  v
Wishlist domain model
  |
  v
Store interface
  |-- Local JSON file
  `-- MongoDB
```

## Program startup

`cmd/main.go` is the executable entry point. It:

1. Creates an Echo server.
2. Enables request logging.
3. Exposes the `css` directory at `/css`.
4. loads the Go templates from `views/*.html`.
5. Initializes the application and its storage provider.
6. Registers the HTTP routes.
7. Starts the server.

The server selects its port in the following order:

1. `PORT`
2. `WEBSITES_PORT`
3. `43067`

The second variable supports environments such as Azure App Service.

## HTTP application

`internal/app/app.go` connects HTTP requests to the domain and storage layers.

During initialization, the application selects a storage implementation:

- When `STORE_TYPE=cloud`, it creates a MongoDB store using `CONN_STR`,
  `DB_NAME`, and `DB_COLLECTION`.
- Otherwise, it uses the local `data/wishlist.json` file.

The selected store loads the wishlist into memory. HTTP requests work with this
in-memory value, and mutations are then written to the selected store.

### Routes

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/` | Render the main wishlist page |
| `GET` | `/wishlist` | Return the complete wishlist as JSON |
| `GET` | `/wishlist/refresh` | Reload the wishlist from storage |
| `POST` | `/wishlist` | Replace and save the complete wishlist |
| `PUT` | `/wishitem` | Create an item, or update an item with the same ID |
| `POST` | `/wishitem` | Update an existing item |
| `POST` | `/wishitem/:id/buy` | Mark an item as purchased |

The Bruno request collection in `docs/requests` can be used to exercise several
of these endpoints manually.

## Domain model

### WishItem

`internal/domain/wishitem.go` defines an individual gift idea. A wish item has:

- An ID
- A name and display title
- A description
- An item type
- A store URL
- An image URL
- A flag indicating whether it was purchased

### Wishlist

`internal/domain/wishlist.go` groups items into three slices:

- T-shirts
- Books
- Other items

The domain model provides operations for adding, finding, updating, and marking
items as purchased.

An item's `ItemType` determines its collection:

- `t-shirt` selects the T-shirts collection.
- `book` selects the Books collection.
- Any other value selects the Other collection.

## Storage

`internal/storage/store.go` defines the storage boundary:

```go
type Store interface {
    Load() domain.Wishlist
    SaveWishList(payload domain.Wishlist) error
}
```

This interface allows the application layer to use either storage implementation
without containing file- or database-specific logic.

### File storage

`internal/storage/filestore.go` reads the complete wishlist from a JSON file and
overwrites that file when the list changes. The default file is
`data/wishlist.json`.

### MongoDB storage

`internal/storage/mongostore.go` reads and replaces one MongoDB document. The
current implementation creates a connection for each operation and selects the
document using a fixed ID.

## Front end

`views/index.html` is a server-rendered Go template. It renders the wishlist
categories and uses a reusable `wishlistitem` template for individual gift
cards. Styling is defined in `css/index-v1.css`.

The purchase action uses HTMX:

```text
Visitor clicks "Comprei"
  |
  v
POST /wishitem/{id}/buy
  |
  v
Server marks and saves the item
  |
  v
Server renders one updated item
  |
  v
HTMX replaces the existing card
```

This provides an interactive update without a custom JavaScript application or
a full-page reload.

## Development and deployment

- `.air.toml` configures local hot reloading with Air.
- `Dockerfile` creates an Alpine-based application image containing the
  executable, templates, CSS, and initial JSON data.
- `.github/workflows/build.yml` defines a reusable Windows build workflow aimed
  at an Azure deployment.

The executable currently uses relative paths for its templates, CSS, and data.
It therefore expects to run from a directory containing `views`, `css`, and
`data`.

