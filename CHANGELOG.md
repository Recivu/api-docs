# Changelog

All notable changes to the Recivu API will be documented in this file.

## [Unreleased]

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

## [2026-04-16]

### Added

- Webhook signature verification (`x-recivu-signature`) documentation
