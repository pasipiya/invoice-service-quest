-- Invoice flow schema.
-- Synthetic data only. No real customer, product or pricing data appears here.

CREATE TABLE products (
    id               BIGSERIAL PRIMARY KEY,
    sku              TEXT   NOT NULL UNIQUE,
    name             TEXT   NOT NULL,
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0)
);

CREATE TABLE tax_rules (
    id      BIGSERIAL PRIMARY KEY,
    region  TEXT NOT NULL UNIQUE,
    rate_bp INT  NOT NULL CHECK (rate_bp >= 0)  -- basis points, e.g. 2000 = 20.00%
);

CREATE TABLE orders (
    id            BIGSERIAL PRIMARY KEY,
    customer_name TEXT        NOT NULL,
    region        TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE line_items (
    id         BIGSERIAL PRIMARY KEY,
    order_id   BIGINT NOT NULL REFERENCES orders (id),
    product_id BIGINT NOT NULL REFERENCES products (id),
    quantity   INT    NOT NULL CHECK (quantity > 0),
    -- ship-to region for this line; may differ from the order region
    region     TEXT   NOT NULL
);

CREATE INDEX line_items_order_id_idx ON line_items (order_id);
