package domain

type WishItem struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ItemType     string `json:"itemtype"`
	ShopUrl      string `json:"shopurl"`
	WasPurchased bool   `json:"waspurchased"`
	ImgSource    string `json:"imgsource"`
}
