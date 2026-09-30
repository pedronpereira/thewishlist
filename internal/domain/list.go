package domain

// List is a named collection of WishItems (e.g. "Pedro", "Pedro's Christmas
// list", "Wife"). Its items live separately, as a Wishlist keyed by Slug.
type List struct {
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	Icon             string `json:"icon"`
	IsAdminRecipient bool   `json:"isadminrecipient"`
	IsDefault        bool   `json:"isdefault"`
	ItemCount        int    `json:"-"` // populated by Store.GetLists for display; never persisted or marshaled
}

// ListWithItems bundles a list's metadata with its items, for bulk
// import/export (GET/POST /wishlist) and as FileStore's on-disk shape.
type ListWithItems struct {
	List
	Items []WishItem `json:"items"`
}
