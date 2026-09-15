package contracttests

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestContract walks every documented partner operation once, in the order a
// partner would use them, with a sandbox key. Subtests share state (ids come
// from earlier calls) so they must run in sequence; each one is still named
// after the operation it covers so a failure reads as "GET /receipts/{id}
// drifted", not "step 7 failed".
//
// Side effects on staging: one sandbox company, one sandbox employee, one
// sandbox receipt and one sandbox merchant delegation per run, all attributed
// to the contract-test partner and soft-deleted where the API allows it. No
// webhook is configured, so nothing leaves the platform.
func TestContract(t *testing.T) {
	c := newClient(t)
	run := time.Now().UTC().Format("20060102-150405")

	var (
		companyID   string
		employeeID  string
		receiptID   string
		merchantVAT = italianVAT(time.Now().UnixNano()/1e6 + 1)
		companyVAT  = italianVAT(time.Now().UnixNano() / 1e6)
	)

	t.Run("POST /company", func(t *testing.T) {
		var out struct {
			CompanyID string `json:"company_id"`
			Status    string `json:"status"`
		}
		expect(t, c.call(t, http.MethodPost, "/company", map[string]any{
			"vat_number":   companyVAT,
			"company_name": "Contract test " + run,
			"company_address": map[string]any{
				"address": "Via Newton 104", "postal_code": "50018", "city": "Scandicci", "province": "FI", "country": "IT",
			},
			"company_transmission_channel": map[string]any{"sdi_code": "0000000"},
			"company_contacts":             map[string]any{"company_email": "contract-tests@recivu.it"},
		}), http.StatusCreated).JSON(t, &out)
		if out.CompanyID == "" {
			t.Fatal("company_id is empty")
		}
		companyID = out.CompanyID
	})
	if companyID == "" {
		t.Fatal("cannot continue without a company")
	}

	t.Run("GET /company/{id}", func(t *testing.T) {
		expect(t, c.call(t, http.MethodGet, "/company/"+companyID, nil), http.StatusOK)
	})

	t.Run("PUT /company/{id}", func(t *testing.T) {
		expect(t, c.call(t, http.MethodPut, "/company/"+companyID, map[string]any{
			"company_name": "Contract test " + run + " (updated)",
			"company_address": map[string]any{
				"address": "Via Newton 105", "postal_code": "50018", "city": "Scandicci", "province": "FI", "country": "IT",
			},
			"company_transmission_channel": map[string]any{"sdi_code": "0000000"},
			"company_contacts":             map[string]any{"company_email": "contract-tests@recivu.it"},
		}), http.StatusOK)
	})

	t.Run("POST /company/{id}/webhook-secret/rotate", func(t *testing.T) {
		// Documented outcomes: 200 with the new secret, or 409 when the company
		// has no webhook_url (this one has none). Both are contract-valid; the
		// call already checked the body shape for whichever came back.
		r := c.call(t, http.MethodPost, "/company/"+companyID+"/webhook-secret/rotate", nil)
		if r.Status != http.StatusOK && r.Status != http.StatusConflict {
			t.Fatalf("status %d, expected 200 or 409", r.Status)
		}
	})

	t.Run("POST /employee", func(t *testing.T) {
		var out struct {
			EmployeeID string `json:"employee_id"`
		}
		expect(t, c.call(t, http.MethodPost, "/employee", map[string]any{
			"company_id": companyID,
			"first_name": "Contract",
			"last_name":  "Test " + run,
		}, withHeader("Idempotency-Key", "contract-test-"+run)), http.StatusCreated).JSON(t, &out)
		if out.EmployeeID == "" {
			t.Fatal("employee_id is empty")
		}
		employeeID = out.EmployeeID
	})
	if employeeID == "" {
		t.Fatal("cannot continue without an employee")
	}

	t.Run("PUT /employee/{id}", func(t *testing.T) {
		// company_id is optional on update and deliberately omitted: with a
		// test key, passing the employee's own sandbox company answers
		// 404 "Target company not found" (TECH-1077); restore it once fixed.
		expect(t, c.call(t, http.MethodPut, "/employee/"+employeeID, map[string]any{
			"first_name": "Contract",
			"last_name":  "Test " + run + " (updated)",
		}), http.StatusOK)
	})

	t.Run("POST /receipt", func(t *testing.T) {
		img, err := os.ReadFile("testdata/receipt.jpg")
		if err != nil {
			t.Fatal(err)
		}
		var out struct {
			ReceiptID string `json:"receipt_id"`
		}
		expect(t, c.call(t, http.MethodPost, "/receipt", map[string]any{
			"receipt_image":          base64.StdEncoding.EncodeToString(img),
			"receipt_image_filename": "receipt.jpg",
			"employee_id":            employeeID,
			"company_id":             companyID,
			"receipt": map[string]any{
				"receipt_header": map[string]any{
					"merchant_name": "Bar Contract Test",
					"merchant_vat":  merchantVAT,
				},
				"receipt_body": map[string]any{
					"products": []map[string]any{{
						"product_description": "Caffè",
						"product_quantity":    1,
						"product_total":       1.20,
						"product_vat":         0.11,
					}},
					"receipt_total": 1.20,
					"receipt_vat":   0.11,
				},
				"receipt_footer": map[string]any{
					"receipt_date_time": time.Now().UTC().Format(time.RFC3339),
					"receipt_number":    "0001-" + run,
					"rt_number":         "99MEY000001",
				},
			},
		}), http.StatusOK).JSON(t, &out)
		if out.ReceiptID == "" {
			t.Fatal("receipt_id is empty")
		}
		receiptID = out.ReceiptID
	})
	// The receipt subtree depends on POST /receipt, which also depends on the
	// object storage being up (the image is uploaded before anything is
	// persisted). When it fails, its own subtest is red; the rest of the
	// surface is still checked instead of hiding behind it.
	needsReceipt := func(t *testing.T) {
		t.Helper()
		if receiptID == "" {
			t.Skip("no receipt: POST /receipt failed above")
		}
	}

	t.Run("GET /receipts/{id} (pending)", func(t *testing.T) {
		needsReceipt(t)
		c.call(t, http.MethodGet, "/receipts/"+receiptID, nil)
	})

	t.Run("POST /sandbox/receipts/{id}/trigger", func(t *testing.T) {
		needsReceipt(t)
		expect(t, c.call(t, http.MethodPost, "/sandbox/receipts/"+receiptID+"/trigger", map[string]any{
			"company_id": companyID,
			"event":      "completed",
		}), http.StatusAccepted)
	})

	t.Run("GET /receipts/{id} (completed)", func(t *testing.T) {
		needsReceipt(t)
		var out struct {
			MachineStatus string `json:"machine_status"`
		}
		expect(t, c.call(t, http.MethodGet, "/receipts/"+receiptID, nil), http.StatusOK).JSON(t, &out)
		if out.MachineStatus != "completed" {
			t.Errorf("machine_status = %q after a completed trigger, want completed", out.MachineStatus)
		}
	})

	t.Run("GET /invoices_download/{id}", func(t *testing.T) {
		needsReceipt(t)
		r := c.call(t, http.MethodGet, "/invoices_download/"+receiptID, nil)
		if r.Status == http.StatusOK && len(r.Body) == 0 {
			t.Error("200 with an empty PDF body")
		}
	})

	t.Run("POST /receipts/{id}/approve", func(t *testing.T) {
		needsReceipt(t)
		// Only receipts awaiting partner review can be approved; a completed
		// one is a documented 409 (or 200 if approval is a no-op). Either is
		// contract-valid; what matters is the shape.
		c.call(t, http.MethodPost, "/receipts/"+receiptID+"/approve", nil)
	})

	t.Run("POST /receipts/{id}/reject", func(t *testing.T) {
		needsReceipt(t)
		c.call(t, http.MethodPost, "/receipts/"+receiptID+"/reject", map[string]any{"reason": "Contract test"})
	})

	t.Run("POST /merchant", func(t *testing.T) {
		expect(t, c.call(t, http.MethodPost, "/merchant", map[string]any{
			"vat_number":    merchantVAT,
			"merchant_name": "Bar Contract Test " + run,
			"merchant_address": map[string]any{
				"address": "Via Roma 1", "postal_code": "00100", "city": "Roma", "province": "RM", "country": "IT",
			},
		}), http.StatusCreated)
	})

	t.Run("GET /merchant/{vat_number}", func(t *testing.T) {
		expect(t, c.call(t, http.MethodGet, "/merchant/"+merchantVAT, nil), http.StatusOK)
	})

	t.Run("GET /reports/receipts", func(t *testing.T) {
		expect(t, c.call(t, http.MethodGet, "/reports/receipts?"+reportRange(), nil), http.StatusOK)
	})

	t.Run("GET /reports/invoices", func(t *testing.T) {
		expect(t, c.call(t, http.MethodGet, "/reports/invoices?"+reportRange(), nil), http.StatusOK)
	})

	// Negative paths: the error envelope is part of the contract too.
	t.Run("GET /receipts/{id} unknown → 404", func(t *testing.T) {
		r := c.call(t, http.MethodGet, "/receipts/rcp_contract_test_does_not_exist", nil)
		if r.Status != http.StatusNotFound {
			t.Fatalf("status %d, want 404", r.Status)
		}
	})

	t.Run("POST /receipt empty body → 400", func(t *testing.T) {
		r := c.call(t, http.MethodPost, "/receipt", nil, withRawBody([]byte(`{}`)))
		if r.Status != http.StatusBadRequest {
			t.Fatalf("status %d, want 400", r.Status)
		}
	})

	t.Run("no API key → 401", func(t *testing.T) {
		r := c.call(t, http.MethodGet, "/company/"+companyID, nil, withoutAuth())
		if r.Status != http.StatusUnauthorized {
			t.Fatalf("status %d, want 401", r.Status)
		}
	})

	// Cleanup last, and still validated: DELETE has a contract too.
	t.Run("DELETE /employee/{id}", func(t *testing.T) {
		expect(t, c.call(t, http.MethodDelete, "/employee/"+employeeID, nil), http.StatusOK)
	})

	t.Run("DELETE /company/{id}", func(t *testing.T) {
		expect(t, c.call(t, http.MethodDelete, "/company/"+companyID, nil), http.StatusOK)
	})
}

// expect fails the subtest when the (already contract-valid) response has a
// different status than the happy path the flow needs to continue.
func expect(t *testing.T, r response, status int) response {
	t.Helper()
	if r.Status != status {
		t.Fatalf("status %d, want %d\nbody: %s", r.Status, status, truncate(r.Body, 600))
	}
	return r
}

// reportRange is the last 7 days as the `from`/`to` date query the reports need.
func reportRange() string {
	now := time.Now().UTC()
	return fmt.Sprintf("from=%s&to=%s", now.AddDate(0, 0, -7).Format("2006-01-02"), now.Format("2006-01-02"))
}

// italianVAT builds a checksum-valid partita IVA from a seed so every run uses
// a fresh one and never hits the documented 409 on re-registration. The
// prefix 99 keeps it out of the range real companies get.
func italianVAT(seed int64) string {
	body := "99" + fmt.Sprintf("%08d", seed%100000000)
	sum := 0
	for i, ch := range body {
		d, _ := strconv.Atoi(string(ch))
		if i%2 == 1 { // even positions (1-based) are doubled, Luhn-style
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	check := (10 - sum%10) % 10
	return "IT" + body + strconv.Itoa(check)
}
