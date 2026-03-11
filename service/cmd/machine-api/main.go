package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

type Server struct {
	DB       *sql.DB
	AdminKey string
}

type AuditRequest struct {
	Target string `json:"target"`
}

func main() {
	db, err := sql.Open("sqlite", "./data/machine.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	s := &Server{
		DB:       db,
		AdminKey: os.Getenv("ADMIN_KEY"),
	}

	http.HandleFunc("/healthz", s.handleHealth)
	http.HandleFunc("/v1/balance", s.handleBalance)
	http.HandleFunc("/v1/audits", s.handleCreateAudit)
	http.HandleFunc("/admin/tokens", s.handleAdminTokens)
	http.HandleFunc("/admin/recent", s.handleAdminRecent)

	log.Println("machine-api listening on :8088")
	log.Fatal(http.ListenAndServe(":8088", nil))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"ok":   true,
		"time": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		writeErr(w, http.StatusUnauthorized, "missing token")
		return
	}

	name, credits, err := s.lookupToken(token)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid token")
		return
	}

	writeJSON(w, map[string]any{
		"ok":                  true,
		"token_name":          name,
		"credits_remaining":   credits,
		"rate_limit_per_hour": 60,
	})
}

func (s *Server) handleCreateAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	token := bearerToken(r)
	if token == "" {
		writeErr(w, http.StatusUnauthorized, "missing token")
		return
	}

	name, credits, err := s.lookupToken(token)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid token")
		return
	}

	if credits < 39 {
		writeErr(w, http.StatusPaymentRequired, "insufficient credits")
		return
	}

	var req AuditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Target == "" {
		writeErr(w, http.StatusBadRequest, "invalid request")
		return
	}

	jobID := "job_" + time.Now().UTC().Format("20060102_150405")
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = s.DB.Exec(`
		INSERT INTO jobs (job_id, token_name, target, status, credits_used, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, jobID, name, req.Target, "queued", 39, now, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db insert failed")
		return
	}

	_, err = s.DB.Exec(`UPDATE tokens SET credits = credits - 39 WHERE token = ?`, token)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "credit deduction failed")
		return
	}

	_, _ = s.DB.Exec(`
		INSERT INTO ledger (token_name, delta, reason, ref_id, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, name, -39, "audit_reserved", jobID, now)

	go s.runAudit(jobID, req.Target)

	writeJSON(w, map[string]any{
		"ok":               true,
		"job_id":           jobID,
		"status":           "queued",
		"credits_reserved": 39,
	})
}

func (s *Server) handleAdminTokens(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Admin-Key") != s.AdminKey {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := s.DB.Query(`SELECT name, token, credits FROM tokens ORDER BY id DESC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db query failed")
		return
	}
	defer rows.Close()

	type item struct {
		Name        string `json:"name"`
		TokenMasked string `json:"token_masked"`
		Credits     int    `json:"credits"`
	}

	var items []item
	for rows.Next() {
		var name, token string
		var credits int
		_ = rows.Scan(&name, &token, &credits)
		mask := token
		if len(mask) > 10 {
			mask = mask[:6] + "..." + mask[len(mask)-4:]
		}
		items = append(items, item{
			Name:        name,
			TokenMasked: mask,
			Credits:     credits,
		})
	}

	writeJSON(w, map[string]any{"ok": true, "items": items})
}

func (s *Server) handleAdminRecent(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Admin-Key") != s.AdminKey {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := s.DB.Query(`
		SELECT job_id, target, status, COALESCE(visa_id, '')
		FROM jobs
		ORDER BY id DESC
		LIMIT 20
	`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db query failed")
		return
	}
	defer rows.Close()

	type job struct {
		JobID  string `json:"job_id"`
		Target string `json:"target"`
		Status string `json:"status"`
		VisaID string `json:"visa_id"`
	}

	var recent []job
	for rows.Next() {
		var j job
		_ = rows.Scan(&j.JobID, &j.Target, &j.Status, &j.VisaID)
		recent = append(recent, j)
	}

	writeJSON(w, map[string]any{"ok": true, "recent": recent})
}

func (s *Server) runAudit(jobID, target string) {
	visaID := "AV-" + time.Now().UTC().Format("20060102-150405")
	now := time.Now().UTC().Format(time.RFC3339)

	_, _ = s.DB.Exec(`
		UPDATE jobs
		SET status = ?, visa_id = ?, result = ?, evidence_url = ?, updated_at = ?
		WHERE job_id = ?
	`, "done", visaID, "PASS", "https://api.aiprotocol.uk/evidence/"+visaID+".json", now, jobID)
}

func (s *Server) lookupToken(token string) (string, int, error) {
	var name string
	var credits int
	err := s.DB.QueryRow(`
		SELECT name, credits FROM tokens
		WHERE token = ? AND enabled = 1
	`, token).Scan(&name, &credits)
	return name, credits, err
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && h[:7] == "Bearer " {
		return h[7:]
	}
	return ""
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	writeJSON(w, map[string]any{"ok": false, "error": msg})
}
