# Changelog

All notable changes to the Recivu API are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the API follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html). The current version is mirrored in `info.version` of `openapi.yaml`. Breaking changes are tagged **BREAKING** and only ship in a `MAJOR` bump — partners can pin to a `MAJOR` version line and expect no integration-breaking changes within it.

## [Unreleased]

## [3.0.0] - 2026-07-01

### Changed

- **BREAKING:** Test (sandbox) submissions to `POST /receipt` no longer fire any webhook automatically. A test receipt is persisted to the sandbox, but the `receipt.conversion.completed` (and other) callbacks are now driven explicitly from the partner portal (or directly via `POST /sandbox/receipts/{id}/trigger`). Partners that relied on a webhook arriving on test submission must switch to triggering events themselves.
- Sandbox `receipt.conversion.completed` callbacks now always carry mock `merchant_name`, `merchant_vat`, and `recovered_vat` values (echoing the submitted receipt's stored fields where available, otherwise hardcoded placeholders), so partners can validate handling of those fields end-to-end.

### Removed

- **BREAKING:** Removed the `sandbox_scenario` field and the reserved `merchant_vat` scenario sentinels (`IT0000000000{2..5}`) from `POST /receipt`. Sandbox outcomes are now selected per trigger via `POST /sandbox/receipts/{id}/trigger` (`event`: `completed` | `failed` | `reverted`, with an optional `count`).
- **BREAKING:** Removed the `payload_variant` field from `POST /receipt`. Payload-shape selection remains available on `POST /sandbox/receipts/{id}/trigger`.

## [2.5.0] - 2026-06-30

### Added

- Sandbox scenario engine on `POST /receipt` (test API key): optional `sandbox_scenario` field (`completed`, `failed`, `unknown_id`, `double_send`, `storno`) to deterministically choose the simulated webhook outcome, plus reserved `merchant_vat` sentinels (`IT0000000000{2..5}`) for clients that cannot set the field.
- Optional `payload_variant` field on `POST /receipt` (`current` | `expected`) selecting the `receipt.conversion.completed` payload shape; the `expected` variant carries a new `schema_version` field on `WebhookPayload`.
- `POST /sandbox/receipts/{id}/trigger` endpoint (test API key only) to fire a webhook event (`completed` | `failed` | `reverted`, with an optional `count` for double-send) for an existing sandbox receipt out-of-band — for replays and lifecycle transitions such as storno.

### Changed

- The request environment (Test vs Live) is now derived from the API key — a test key routes to the sandbox, a live key to production. The `type` field on `POST /receipt` is **deprecated**: it is still accepted during a 30-day partner-notice window but ignored for environment-scoped keys, and will be removed in a future `MAJOR` version.

## [2.4.0] - 2026-06-23

### Added

- `receipt.conversion.failed` webhook is now delivered in production. It fires when a receipt is archived — either by staff (e.g. illegible scontrino) or automatically as a duplicate — with the archive reason in the `error` field. Previously this event was documented but never sent.

### Changed

- `recovered_vat` on the `receipt.conversion.completed` webhook now reflects the actual IVA recovered from the receipt (previously always `0`). This includes the mock `completed` webhook fired in `Test` mode, which is now populated with the submitted `merchant_name`/`merchant_vat` and the `recovered_vat` from the request body's `receipt_vat`.

## [2.3.0] - 2026-05-20

### Added

- Optional `email` and `phone_number` fields on `POST /employee` request body (`CreateEmployeeRequest`). Neither field is required for partner integrations; both are stored and returned in `GET /company/{id}` employee listings (`EmployeeSummary`).
- `merchant_name` and `merchant_vat` fields on the `receipt.conversion.completed` webhook payload, giving partners the supplier ragione sociale and P.IVA for each recovered invoice.
- `receipt.conversion.completed` webhook now fires for **Live** receipts at the moment the merchant confirms invoice emission (previously fired only for Test receipts).
- `receipt_image_url` field on `GET /receipts/{id}` responses (both `200` and `202`). Returns the URL of the original receipt image uploaded with `POST /receipt`, or an empty string when no image was uploaded. In `Test` mode (`?type=Test`) a stub URL is returned.

## [2.1.0] - 2026-04-29

### Fixed

- `POST /company` `409 Conflict` response now includes `company_id` when the duplicate VAT number is already registered by the requesting partner, letting partners recover the existing company id without an extra `GET` lookup. The field is omitted when the duplicate belongs to a different partner.

## [2.0.0] - 2026-04-27

> Versions before `2.0.0` were dated rather than numbered. The 2026-04-27 release shipped breaking changes (HTTP status code shifts, type coercion of monetary fields, schema removals) and is retroactively labelled `2.0.0`.

### Added

- `POST /company/{id}/webhook-secret/rotate` endpoint for regenerating a company's `webhook_signing_secret`. Use when the original secret is lost or suspected of being compromised. Returns `409 Conflict` when the company has no `webhook_url` configured.
- `webhook_signing_secret` is now also returned on `PUT /company/{id}` when the update first sets a `webhook_url` on a company that previously had none, closing the gap where partners who added their webhook after registration had no way to obtain a signing secret.
- `receipt.conversion.reverted` webhook event: fired with `status: completed` when a credit note is successfully issued after a partner rejects a delivered receipt.
- `type` field (`Live` or `Test`) on `POST /company`, defaulting to `Live` when omitted. Lets partners register a sandbox `Test` company alongside a `Live` one with the same VAT number without conflict. Also returned on `GET /company/{id}`.
- End-to-end Test flow on `POST /receipt`. When the request `type` is `Test`, the receipt is not persisted and a signed `receipt.conversion.completed` webhook is fired to the target company's `webhook_url` (if configured) so partners can validate their signature verification and callback handling.
- `?type=Test` query parameter on `GET /receipts/{id}` — returns a mock completed response without database access, letting partners exercise status polling in test mode with any receipt id.

### Changed

- **BREAKING:** Monetary and quantity fields (`receipt_total`, `receipt_vat`, `product_total`, `product_vat`, `product_quantity`, `product_absolute_discount`, `product_percent_discount`, `invoice_total`) changed from `string` to `number` (float) to match implementation.
- **BREAKING:** `POST /company` success response changed from `200` to `201 Created`.
- **BREAKING:** `POST /employee` success response changed from `200` to `201 Created`.
- `PUT /employee/{id}` now accepts optional `company_id` field for employee reassignment between companies.
- `GET /receipts/{id}` 404 response now uses standard `ErrorResponse` schema.
- `GET /invoices_download/{id}` 404 response now uses standard `ErrorResponse` schema.

### Removed

- **BREAKING:** `GetInvoiceStatusErrorResponse` schema (replaced by standard `ErrorResponse`).
- **BREAKING:** `GetInvoiceFileErrorResponse` schema (replaced by standard `ErrorResponse`).

### Fixed

- `POST /company` no longer returns `409 Conflict` when re-registering a company whose previous record was soft-deleted via `DELETE /company/{id}`. A new company with a new `id` is created. Existing employees and receipts attached to the deleted company are not migrated.

## [1.1.0] - 2026-04-16

### Added

- Webhook signature verification (`x-recivu-signature`) documentation.

## [1.0.0]

- Initial public release.
