package main

import (
	"fmt"
	"log"
	"net/http"
	"slices"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func handlePing(w http.ResponseWriter, r *http.Request) {
	writeText(w, http.StatusOK, fmt.Sprintf("PONG - %s", time.Now().Format(time.RFC3339)))
}

func handleLogin(w http.ResponseWriter, r *http.Request) {

	// Restrict to Post
	if r.Method != http.MethodPost {
		writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Decode Body
	var body struct {
		Identifier string
		Password   string
	}
	if err := GetArguments(r, &body); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid request")
		return
	}

	// Fetch User
	var passwordHash string
	var userID string
	err := DB.QueryRow(r.Context(), `SELECT id, password FROM users WHERE email = $1 OR cell = $1`,
		body.Identifier).Scan(&userID, &passwordHash)
	if err != nil {
		writeText(w, http.StatusUnauthorized, "Invalid email/cell or password")
		return
	}

	// Compare Password and Hash
	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(body.Password))
	if err != nil {
		writeText(w, http.StatusUnauthorized, "Invalid email/cell or password")
		return
	}

	// Success
	writeJSON(w, http.StatusOK, map[string]string{"id": userID})
}

func handleRegisterUser(w http.ResponseWriter, r *http.Request) {

	// Restrict to Post
	if r.Method != http.MethodPost {
		writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Decode Body
	var body struct {
		Email    string
		Cell     string
		Password string
		Name     string
	}
	if err := GetArguments(r, &body); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validation
	if body.Email == "" || body.Password == "" || body.Cell == "" {
		writeText(w, http.StatusBadRequest, "Email, Password or Cell empty")
		return
	}

	// Hashing
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeText(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Register
	if _, err := DB.Exec(r.Context(), `INSERT INTO users (cell, email, password, name) VALUES ($1, $2, $3, $4);`,
		body.Cell, body.Email, hashedPassword, body.Name); err != nil {
		writeText(w, http.StatusInternalServerError, "Failed to register user")
		return
	}

	// Success
	w.WriteHeader(http.StatusCreated)
}

func handleRegisterPivot(w http.ResponseWriter, r *http.Request) {

	// Restrict to Post
	if r.Method != http.MethodPost {
		writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Decode Body
	var body struct {
		Name string
		User string
		Imei string
	}
	if err := GetArguments(r, &body); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validation
	if body.Name == "" || body.User == "" || body.Imei == "" {
		writeText(w, http.StatusBadRequest, "Name, User or Imei empty")
		return
	}

	// Register
	if _, err := DB.Exec(r.Context(), `INSERT INTO pivots (name, "user", imei) VALUES ($1, $2, $3);`,
		body.Name, body.User, body.Imei); err != nil {
		writeText(w, http.StatusInternalServerError, "Failed to register pivot")
		return
	}

	// Success
	w.WriteHeader(http.StatusCreated)
}

func handleGetUserPivots(w http.ResponseWriter, r *http.Request) {

	// Restrict to Get
	if r.Method != http.MethodGet {
		writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get Query Arguments
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeText(w, http.StatusBadRequest, "user_id is required")
		return
	}

	// Get user pivots
	rows, err := DB.Query(r.Context(), `SELECT id, name FROM pivots WHERE "user" = $1`, userID)
	if err != nil {
		writeText(w, http.StatusInternalServerError, "Failed to fetch user pivots")
		return
	}
	defer rows.Close()

	// Return
	type pivot struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var pivots []pivot

	for rows.Next() {
		var p pivot
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			writeText(w, http.StatusInternalServerError, "Internal Server Error")
			return
		}
		pivots = append(pivots, p)
	}

	writeJSON(w, http.StatusOK, pivots)
}

func handleCommand(w http.ResponseWriter, r *http.Request) {

	// Restrict to Post
	if r.Method != http.MethodPost {
		writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Decode Body
	var body struct {
		PivotId string  `json:"pivot_id"`
		Command string  `json:"command"`
		Payload *string `json:"payload"`
	}
	if err := GetArguments(r, &body); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validation
	if !slices.Contains(Commands, body.Command) {
		writeText(w, http.StatusBadRequest, "Invalid command")
	}

	// Insert (created_at is automatically populated)
	if _, err := DB.Exec(r.Context(), `INSERT INTO pivot_command_queue (pivot_id, command, payload) VALUES ($1, $2, $3);`,
		body.PivotId, body.Command, body.Payload); err != nil {
		writeText(w, http.StatusInternalServerError, "Failed to queue command")
		return
	}

	// Success
	w.WriteHeader(http.StatusOK)
}

func handlePivotStatus(w http.ResponseWriter, r *http.Request) {

	// Restrict to GET
	if r.Method != http.MethodGet {
		writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Decode Body
	var args struct {
		PivotId string `json:"pivot_id"`
	}
	if err := GetArguments(r, &args); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid arguments")
		return
	}

	// Fetch Status
	var pivotStatus struct {
		PositionDeg float64 `db:"position_deg"`
		SpeedPct    float64 `db:"speed_pct"`
		Direction   string  `db:"direction"`
		Wet         bool    `db:"wet"`
		Status      string  `db:"status"`
		Battery     float64 `db:"battery_pct"`
	}
	err := DB.QueryRow(r.Context(), `SELECT position_deg, speed_pct, direction::text, wet, status::text, battery_pct
        FROM pivot_status
        WHERE pivot_id=$1`,
		args.PivotId).Scan(
		&pivotStatus.PositionDeg,
		&pivotStatus.SpeedPct,
		&pivotStatus.Direction,
		&pivotStatus.Wet,
		&pivotStatus.Status,
		&pivotStatus.Battery)
	if err != nil {
		log.Println(err)
		writeText(w, http.StatusNotFound, "Pivot Not Found")
		return
	}

	// Success
	writeJSON(w, http.StatusOK, pivotStatus)
}
