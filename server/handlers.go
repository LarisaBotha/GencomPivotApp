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
	if r.Method != http.MethodPost && r.Method != http.MethodOptions {
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
	if r.Method != http.MethodPost && r.Method != http.MethodOptions {
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
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Internal server error: %v", err))
		return
	}

	// Register
	if _, err := DB.Exec(r.Context(), `INSERT INTO users (cell, email, password, name) VALUES ($1, $2, $3, $4);`,
		body.Cell, body.Email, hashedPassword, body.Name); err != nil {
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to register user: %v", err))
		return
	}

	// Success
	writeHeader(w, http.StatusCreated)
}

func handleRegisterPivot(w http.ResponseWriter, r *http.Request) {

	// Restrict to Post
	if r.Method != http.MethodPost && r.Method != http.MethodOptions {
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
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to register pivot: %v", err))
		return
	}

	// Success
	writeHeader(w, http.StatusCreated)
}

func handleGetUserPivots(w http.ResponseWriter, r *http.Request) {

	// Restrict to Get
	if r.Method != http.MethodGet && r.Method != http.MethodOptions {
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
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch user pivots: %v", err))
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
			writeText(w, http.StatusInternalServerError, fmt.Sprintf("Internal server error: %v", err))
			return
		}
		pivots = append(pivots, p)
	}

	writeJSON(w, http.StatusOK, pivots)
}

func handleCommand(w http.ResponseWriter, r *http.Request) {

	// Restrict to Post
	if r.Method != http.MethodPost && r.Method != http.MethodOptions {
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
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to queue command: %v", err))
		return
	}

	// Success
	w.WriteHeader(http.StatusOK)
}

func handlePivotStatus(w http.ResponseWriter, r *http.Request) {

	// Restrict to GET
	if r.Method != http.MethodGet && r.Method != http.MethodOptions {
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
		PositionDeg float64 `json:"position_deg" db:"position_deg"`
		SpeedPct    float64 `json:"speed_pct" db:"speed_pct"`
		Direction   string  `json:"direction" db:"direction"`
		Wet         bool    `json:"wet" db:"wet"`
		Status      string  `json:"status" db:"status"`
		Battery     float64 `json:"battery_pct" db:"battery_pct"`
	}
	err := DB.QueryRow(r.Context(),
		`SELECT 
			position_deg, 
			speed_pct, 
			direction::text, 
			wet, 
			status::text, 
			battery_pct
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
		log.Println("ERR STAT", err)
		writeText(w, http.StatusNotFound, "Pivot Not Found")
		return
	}

	// Success
	writeJSON(w, http.StatusOK, pivotStatus)
}

func handlePivotTimerSections(w http.ResponseWriter, r *http.Request) {

    if r.Method == http.MethodGet {
        var args struct {
            PivotId string `json:"pivot_id"`
        }
        if err := GetArguments(r, &args); err != nil {
            writeText(w, http.StatusBadRequest, "Invalid arguments")
            return
        }

        type TimerSection struct {
            Serial   int     `json:"serial"`
            SpeedPct float64 `json:"timer_pct"`
            Label    *string `json:"label"`
            Angle    float64 `json:"angle"`
        }

        rows, err := DB.Query(r.Context(),
            `SELECT serial, timer_pct, label, angle_deg 
             FROM pivot_timer_sections 
             WHERE pivot_id = $1 
             ORDER BY serial ASC`, args.PivotId)
        if err != nil {
            writeText(w, http.StatusInternalServerError, "Database error")
            return
        }
        defer rows.Close()

        sections := []TimerSection{}
        for rows.Next() {
            var s TimerSection
            if err := rows.Scan(&s.Serial, &s.SpeedPct, &s.Label, &s.Angle); err != nil {
                writeText(w, http.StatusInternalServerError, "Scan error")
                return
            }
            sections = append(sections, s)
        }
        writeJSON(w, http.StatusOK, sections)
        return
    }

    if r.Method == http.MethodPost {
        var args struct {
            PivotId  string  `json:"pivot_id"`
            Serial   *int    `json:"serial"`
            SpeedPct float64 `json:"timer_pct"`
            Label    *string `json:"label"`
            Angle    float64 `json:"angle"`
        }

        if err := GetArguments(r, &args); err != nil {
            writeText(w, http.StatusBadRequest, "Invalid JSON body")
            return
        }

        _, err := DB.Exec(r.Context(), `
            INSERT INTO pivot_timer_sections (pivot_id, serial, timer_pct, label, angle_deg)
            VALUES ($1, $2, $3, $4, $5)
            ON CONFLICT (pivot_id, serial) 
            DO UPDATE SET 
                timer_pct = EXCLUDED.timer_pct,
                label = EXCLUDED.label,
                angle_deg = EXCLUDED.angle_deg`,
            args.PivotId, args.Serial, args.SpeedPct, args.Label, args.Angle)

        if err != nil {
            writeText(w, http.StatusBadRequest, fmt.Sprintf("Failed to save section: %v", err))
            return
        }

        writeText(w, http.StatusOK, "Section saved successfully")
        return
    }

    writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
}