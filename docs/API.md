# local.bio — reverse-engineering notes

> These notes were gathered by inspecting the public Angular bundle of
> `https://www.local.bio` and by probing public endpoints (no authenticated
> session was used). They describe the HTTP surface the CLI/MCP targets.
> A Chrome instance reachable over CDP (`http://localhost:9222`) is available
> **for dev exploration only** — the shipped CLI never talks to it.

## Hosts / base URLs

The front-end is an Angular SPA. Two REST back-ends are injected at runtime:

| Token (in bundle) | Value                          | Stack                      |
| ----------------- | ------------------------------ | -------------------------- |
| `API_URL`         | `https://www.local.bio/api`    | Legacy LoopBack            |
| `API2_URL`        | `https://www.local.bio/api-v2` | NestJS (current, preferred)|

The modern app talks almost exclusively to **`/api-v2`**. The CLI uses it as
the single base URL (`LOCALBIO_API_BASE`, default `https://www.local.bio/api-v2`).

Multi-tenant: the same back-end serves `local.bio`, `local.direct`,
`local.boutique`. The front sends an `app` header (e.g. `app: local.bio`)
identifying the tenant, plus `Authorization: <token>` once logged in.

## Authentication

- `POST /api-v2/customers/login` — body `{"email": "...", "password": "..."}`.
  Backed by LoopBack: on success it returns an **access-token object**
  `{"id": "<token>", "ttl": 1209600, "created": "...", "userId": "..."}` — the
  session token is the top-level **`id`**. (Failure: `{"error":{"code":"LOGIN_FAILED"}}`.)
  The token is then sent verbatim (no `Bearer ` prefix) in the `Authorization`
  header on subsequent calls. The client extracts it via `findToken`, which also
  tolerates `auth.id` / `token` / `accessToken` shapes.
- `POST /api-v2/customers/logout`
- `GET  /api-v2/customers/me` — current account (401 `{"message":"missing token"}` when anonymous).

## Stores (points de retrait)

- `GET /api-v2/stores/search?lat=<lat>&lng=<lng>` — stores around a coordinate.
  Returns `{ geoTown, stores: [...] }`. Each store has `url` (its id/slug),
  `name`, `address`, `geo`, `type`, `app`, `categories`, `openingHours`…
- `GET /api-v2/stores/loadStore?storeId=<url>` — full detail of a single store.

Because the search needs coordinates, the CLI geocodes a free-text query
(city / postal code) through the official French address API
(`https://api-adresse.data.gouv.fr/search/?q=...`) — same provider whitelisted
in the site CSP.

## Orders

- `GET /api-v2/orders/byCustomer` — list of the customer's orders. Returns each
  order **with its `products[]` embedded**. Orders have **no human number**: they
  are identified by their Mongo `id`. Fields: `{ id, date, storeId, app, payment,
  products[] }`; each product is `{ name, productId, producerId, categoryId,
  quantity, delivery, tax, packaging:{id,name,price,stock,weight} }`. There is no
  total field — it is computed from the lines (+ `payment.appTip`/`storeTip`).
- `GET /api/orders/<id>/getOrderWithPaymentIntent` — order detail. **Legacy host
  only** (`/api`, not `/api-v2`, which 404s). Takes the order `id`; same shape as
  the list entry plus `stripePayments`.
- `GET /api-v2/customers/lastOrder` — most recent order.

The CLI sorts orders newest-first and lets you reference one by 1-based index
(`orders 1` = most recent) or by id / unique id prefix.

## Basket

The basket is **server-side** and shared across devices (browser ↔ mobile).

- **Read**: it is embedded in the customer profile — `GET /api-v2/customers/me`
  → `cart = { id, storeId, payment, products[] }`. (There is no dedicated
  `GET /customers/cart`: that path is POST-only and 404s on GET. `cart` is absent
  from `me` only when the basket is empty.) Each cart product is
  `{ productId, name, quantity, price, delivery, categoryId, producerId, … }`.
- **Write**: `POST /api-v2/customers/cart` with the cart state plus
  `setupPayment` (false except at checkout): `{ storeId, payment, products,
  setupPayment }`. The SPA sends slim product lines
  `{ productId, quantity, packagingId, productName, delivery }`; the server
  resolves price/name and returns the updated `{ customer, mins }`.

The CLI reads `me.cart`, mutates the `products` array (keyed by **product id** —
there is no EAN) and POSTs it back, resolving packaging + next delivery date from
the store catalogue for brand-new lines.

### Write — solved

The winning line shape (captured from the live app via CDP) is **not** the
slim/offer guess. A cart line is:

```jsonc
{
  "productId":"…", "quantity":1.2,            // quantity may be fractional (kg)
  "name":"Ail blanc", "categoryId":"vegetables~garlic", "producerId":"…",
  "img":"…", "about":false, "delivery":"2026-07-01T00:00:00.000Z",
  "price":15, "tax":5.5,
  "status":"onsessionPayment",                // payment mode of the delivery handler
  "alreadyTipped":true, "paymentCostDiscounted":true,
  "fixedStripe":0.25, "discountedFixedStripe":-0.25   // optional; server fills them
}
```

**Packaging-priced products MUST include their `packaging` object** (`{id, name,
price, stock, weight}`) + matching `price` — without it the server falsely
returns `stock:[true,false]`. Weight-priced items omit `packaging` (the
diff-correction loop fills `price`). Delivery date + payment mode come from
**each product's own producer** (`ProducerDeliveryInfo`), since a store mixes
producers with different schedules.

The POST body is `{ storeId, payment, products, setupPayment:false }` where
`payment` is the cart-level object (`{appTip, storeTip, invoicing, deferred,
method}` — `method` = the customer's default payment method). **No** `stock`,
`packaging`, `sku`, `packagingId` or `payed` — sending those is what caused the
earlier 422/500s.

Key insight: the `me.cart` **read** shape **equals** the **write** shape, so the
server's own product objects round-trip verbatim (quantity changes, removals).

For a brand-new line the CLI builds a best guess from the catalogue and uses the
422 diff as a **correction oracle**: the body is
`{"message":[[line, {field:[sent,expected]}], …]}`, so we set each field to its
`expected` value and retry (a couple of passes converge — e.g. `price:[1,15]` →
resend with `price:15` → 201). The only non-correctable field is `stock`, which
means the product is genuinely out of stock for the delivery date → surfaced as
`"<name>" is out of stock for the available delivery date`.

`setupPayment` is always `false` and `payed` is never sent, so **no payment is
ever initiated** (adding to cart does not charge — confirmed against a real
onsessionPayment store).

### Earlier (superseded) analysis

`POST /customers/cart` validates **each line against the live offer** and, on
failure, returns 422 with a per-line diff `[[lineYouSent, {field:[sent,expected]}], …]`.
A valid **new** line is the rich offer line the SPA builds (`Ve(…)` in the
bundle):

```jsonc
{
  "productId": "…", "quantity": 1,
  "packaging": { "id":"…", "name":"…", "price":1.4, "stock":10, "weight":0 }, // FULL object
  "stock": 10,                 // the NUMBER (a boolean fails: true>0 ≠ expected)
  "sku": "<productId>-<packagingId>-<YYYY-MM-DD>",  // = na(): wr(id,pkg)+'-'+day
  "name":"…", "categoryId":"…", "producerId":"…", "img":"…",
  "about":false, "active":true,
  "delivery":"2026-07-01T00:00:00.000Z",
  "tax":5.5, "markup":0, "minSales":0
  // do NOT send "price" (expected null)
}
```

Sending `stock` as the **number** + the full `packaging` object clears the
`stock` check (the catalogue's `packaging.stock` is the source of truth). The
`sku` is `` `${productId}-${packagingId}-${YYYY-MM-DD}` ``.

The last gate is **`payed`** — a per-line "already paid" flag (the SPA groups the
cart as `products.filter(n => !n.status && !n.payed)` = the lines to settle this
session). A fresh line should be `payed:false`/absent, but for some stores the
**server recomputes `payed:true`** for a line built from public REST data and
then rejects it (`payed:[true,false]`). The value the working app sends/derives
depends on server-side payment state that is not exposed by the catalogue or
`me`, so it cannot be reproduced from REST alone. Sending `status` → 500;
sending `payed:true` risks a real charge, so the CLI **never** does that.

> The clean way to finish this is to **capture one real `POST /customers/cart`**
> from a working session (DevTools → Network → add an item → copy the Request
> Payload) and mirror its exact line shape. Attempts to render the catalogue
> under a synthetic auth token fail (the SPA stays in a logged-out-ish state), so
> a live capture is required.

Reads (`basket get`) and re-posting already-valid lines (quantity changes,
removals) always work; adding a brand-new line works wherever the server accepts
`payed:false`.

## Products / catalogue

There is **no EAN/barcode** anywhere in local.bio. A product is identified by
its `id` and sold through one or more `packaging` entries (`{id, name, price,
stock, weight}`); the cart references `productId` + `packagingId`.

A store relays **one or more producers**; the real per-store catalogue + stock is
in `loadStore`, **not** `loadProducer`:

- `GET /api-v2/stores/loadStore?storeId=<ref>` → `producers[*].products`. This is
  the source of truth: every producer currently sold at the store with the
  store's real stock. (`loadProducer` returns a single producer's full,
  often-stale catalogue and misses the other producers — don't use it for the
  catalogue.) Each product: `{ id, name, categoryId, active, img, producerId,
  stock?, packaging[] }`.
- Stock semantics: **packaging-priced** products are orderable when
  `packaging[0].stock > 0`; **weight-priced** products (empty `packaging`) when
  the product-level `stock > 0`. "Any packaging has stock" is wrong (a product
  can have a default packaging at 0 and a variant > 0).

The CLI `search` (default) lists only **truly available** products (`active &&
InStock`); `--all` / `include_unavailable` shows everything.

## Headers summary

```
app: local.bio
Authorization: <token from login>        # only when authenticated
Content-Type: application/json
```
