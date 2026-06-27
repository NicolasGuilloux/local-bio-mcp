package client

// Account is the authenticated customer profile (`GET /customers/me`).
type Account struct {
	ID        string `json:"id,omitempty"`
	Email     string `json:"email,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Phone     string `json:"phone,omitempty"`
	StoreID   string `json:"storeId,omitempty"`
	// Raw keeps every field returned by the API for `--format json`.
	Raw map[string]any `json:"-"`
}

// LoginResponse is what `POST /customers/login` returns. The auth token used as
// the `Authorization` header is read from `auth.id` (falling back to `token`).
type LoginResponse struct {
	Auth struct {
		ID string `json:"id"`
	} `json:"auth"`
	Token    string         `json:"token"`
	Customer map[string]any `json:"customer"`
	Raw      map[string]any `json:"-"`
}

// TokenValue returns the bearer token from a login response.
func (l LoginResponse) TokenValue() string {
	if l.Auth.ID != "" {
		return l.Auth.ID
	}
	return l.Token
}

// Fields lists the top-level keys of the raw response, for diagnostics.
func (l LoginResponse) Fields() string {
	return topKeys(l.Raw)
}

// Geo is a GeoJSON-ish point with convenience lat/lng.
type Geo struct {
	Type        string    `json:"type,omitempty"`
	Coordinates []float64 `json:"coordinates,omitempty"`
	Lat         float64   `json:"lat,omitempty"`
	Lng         float64   `json:"lng,omitempty"`
}

// Address of a store.
type Address struct {
	City     string `json:"city,omitempty"`
	Country  string `json:"country,omitempty"`
	Postcode string `json:"postcode,omitempty"`
	Street   string `json:"street,omitempty"`
}

// Store is a pickup point (point de retrait).
type Store struct {
	URL        string   `json:"url"`
	Slug       string   `json:"slug,omitempty"`
	Name       string   `json:"name"`
	Type       string   `json:"type,omitempty"`
	App        string   `json:"app,omitempty"`
	Address    Address  `json:"address"`
	Geo        Geo      `json:"geo"`
	Categories []string `json:"categories,omitempty"`
}

// StoreSearchResult is the payload of `GET /stores/search`.
type StoreSearchResult struct {
	GeoTown map[string]any `json:"geoTown,omitempty"`
	Stores  []Store        `json:"stores"`
}

// Cart is the customer's server-side basket (lives at `me.cart`).
type Cart struct {
	ID       string         `json:"id,omitempty"`
	StoreID  string         `json:"storeId,omitempty"`
	Payment  map[string]any `json:"payment,omitempty"`
	Products []CartProduct  `json:"products"`
	Raw      map[string]any `json:"-"`
	// RawProducts keeps the server's exact product objects so they can be
	// re-posted verbatim (the write endpoint validates each line strictly).
	RawProducts []map[string]any `json:"-"`
}

// CartProduct is one line of the cart.
type CartProduct struct {
	ProductID   string  `json:"productId"`
	Name        string  `json:"name,omitempty"`
	Quantity    float64 `json:"quantity"`
	Price       float64 `json:"price,omitempty"`
	Delivery    string  `json:"delivery,omitempty"`
	PackagingID string  `json:"packagingId,omitempty"`
	ProducerID  string  `json:"producerId,omitempty"`
	CategoryID  string  `json:"categoryId,omitempty"`
	Tax         float64 `json:"tax,omitempty"`
}

// LineTotal is the price for this cart line.
func (p CartProduct) LineTotal() float64 { return p.Price * p.Quantity }

// Total sums every cart line.
func (c Cart) Total() float64 {
	var t float64
	for _, p := range c.Products {
		t += p.LineTotal()
	}
	return t
}

// Packaging is a sellable unit of a Product (the cart references its id).
type Packaging struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Price  float64 `json:"price"`
	Stock  float64 `json:"stock"`
	Weight float64 `json:"weight,omitempty"`
}

// Product is a catalogue article. local.bio has no EAN/barcode: a product is
// identified by its `id` and sold through one or more `packaging` entries.
type Product struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	CategoryID  string      `json:"categoryId,omitempty"`
	Active      bool        `json:"active"`
	Img         string      `json:"img,omitempty"`
	ProducerID  string      `json:"producerId,omitempty"`
	Stock       float64     `json:"stock,omitempty"` // product-level stock (weight-priced items)
	Packaging   []Packaging `json:"packaging,omitempty"`
}

// IsWeightPriced reports whether the product is sold by weight (no packaging;
// stock and price are product-level) rather than per packaging unit.
func (p Product) IsWeightPriced() bool { return len(p.Packaging) == 0 }

// Price returns the price of the first packaging (0 when none).
func (p Product) Price() float64 {
	if len(p.Packaging) > 0 {
		return p.Packaging[0].Price
	}
	return 0
}

// Unit returns the name of the first packaging (e.g. "botte", "pièce").
func (p Product) Unit() string {
	if len(p.Packaging) > 0 {
		return p.Packaging[0].Name
	}
	return ""
}

// InStock reports whether the product is orderable: for packaging items the
// default (first) packaging must have stock; for weight items the product-level
// stock must be positive.
func (p Product) InStock() bool {
	if len(p.Packaging) > 0 {
		return p.Packaging[0].Stock > 0
	}
	return p.Stock > 0
}

// Order is a customer order. local.bio orders have no human "number": they are
// identified by their Mongo `id`. The order already embeds its product lines.
type Order struct {
	ID       string         `json:"id"`
	Date     string         `json:"date,omitempty"`
	StoreID  string         `json:"storeId,omitempty"`
	App      string         `json:"app,omitempty"`
	Products []OrderProduct `json:"products,omitempty"`
	Payment  OrderPayment   `json:"payment"`
	Raw      map[string]any `json:"-"`
}

// OrderPayment holds the payment/tip breakdown of an order.
type OrderPayment struct {
	AppTip   float64 `json:"appTip,omitempty"`
	StoreTip float64 `json:"storeTip,omitempty"`
	Intent   string  `json:"intent,omitempty"`
	Method   string  `json:"method,omitempty"`
}

// OrderProduct is one article of an order.
type OrderProduct struct {
	Name       string    `json:"name"`
	ProductID  string    `json:"productId,omitempty"`
	ProducerID string    `json:"producerId,omitempty"`
	CategoryID string    `json:"categoryId,omitempty"`
	Quantity   float64   `json:"quantity"`
	Delivery   string    `json:"delivery,omitempty"`
	Tax        float64   `json:"tax,omitempty"`
	Packaging  Packaging `json:"packaging"`
}

// LineTotal is the price for this article line.
func (p OrderProduct) LineTotal() float64 { return p.Packaging.Price * p.Quantity }

// ItemsTotal sums every product line (excluding tips).
func (o Order) ItemsTotal() float64 {
	var s float64
	for _, p := range o.Products {
		s += p.LineTotal()
	}
	return s
}

// Total is the items total plus tips.
func (o Order) Total() float64 { return o.ItemsTotal() + o.Payment.AppTip + o.Payment.StoreTip }
