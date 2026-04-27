# Changelog

All notable changes to the Recivu API will be documented in this file.

## [Unreleased]

## [2026-04-27]

### Added

- `POST /company/{id}/webhook-secret/rotate` endpoint for regenerating a company's `webhook_signing_secret`. Use when the original secret is lost or suspected of being compromised. Returns `409 Conflict` when the company has no `webhook_url` configured.
- `webhook_signing_secret` is now also returned on `PUT /company/{id}` when the update first sets a `webhook_url` on a company that previously had none, closing the gap where partners who added their webhook after registration had no way to obtain a signing secret.
- `receipt.conversion.reverted` webhook event: fired with `status: completed` when a credit note is successfully issued after a partner rejects a delivered receipt.
- `type` field (`Live` or `Test`) on `POST /company`, defaulting to `Live` when omitted. Lets partners register a sandbox `Test` company alongside a `Live` one with the same VAT number without conflict. Also returned on `GET /company/{id}`.
- End-to-end Test flow on `POST /receipt`. When the request `type` is `Test`, the receipt is not persisted and a signed `receipt.conversion.completed` webhook is fired to the target company's `webhook_url` (if configured) so partners can validate their signature verification and callback handling.
- `?type=Test` query parameter on `GET /receipts/{id}` — returns a mock completed response without database access, letting partners exercise status polling in test mode with any receipt id.

### Changed

- Monetary and quantity fields (`receipt_total`, `receipt_vat`, `product_total`, `product_vat`, `product_quantity`, `product_absolute_discount`, `product_percent_discount`, `invoice_total`) changed from `string` to `number` (float) to match implementation
- `POST /company` success response changed from `200` to `201 Created`
- `POST /employee` success response changed from `200` to `201 Created`
- `PUT /employee/{id}` now accepts optional `company_id` field for employee reassignment between companies
- `GET /receipts/{id}` 404 response now uses standard `ErrorResponse` schema
- `GET /invoices_download/{id}` 404 response now uses standard `ErrorResponse` schema

### Removed

- `GetInvoiceStatusErrorResponse` schema (replaced by standard `ErrorResponse`)
- `GetInvoiceFileErrorResponse` schema (replaced by standard `ErrorResponse`)

### Fixed

- `POST /company` no longer returns `409 Conflict` when re-registering a company whose previous record was soft-deleted via `DELETE /company/{id}`. A new company with a new `id` is created. Existing employees and receipts attached to the deleted company are not migrated.

## [2026-04-16]

### Added

- Webhook signature verification (`x-recivu-signature`) documentation
