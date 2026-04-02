package main

import (
	"context"
	"encoding/json"
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

	// Handle Preflight
	if r.Method == http.MethodOptions {
		writeHeader(w, http.StatusOK)
		return
	}

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

	// Handle Preflight
	if r.Method == http.MethodOptions {
		writeHeader(w, http.StatusOK)
		return
	}

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

	// Handle Preflight
	if r.Method == http.MethodOptions {
		writeHeader(w, http.StatusOK)
		return
	}

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

func queuePivotCommand(ctx context.Context, pivotID string, cmd string, payload *string) error {
	_, err := DB.Exec(ctx, `
        INSERT INTO pivot_command_queue (pivot_id, command, payload) 
        VALUES ($1, $2, $3);`,
		pivotID, cmd, payload)
	return err
}

func handleCommand(w http.ResponseWriter, r *http.Request) {

	// Handle Preflight
	if r.Method == http.MethodOptions {
		writeHeader(w, http.StatusOK)
		return
	}

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
	if err := queuePivotCommand(r.Context(), body.PivotId, body.Command, body.Payload); err != nil {
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to queue command: %v", err))
		return
	}
	// if _, err := DB.Exec(r.Context(), `INSERT INTO pivot_command_queue (pivot_id, command, payload) VALUES ($1, $2, $3);`,
	// 	body.PivotId, body.Command, body.Payload); err != nil {
	// 	writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to queue command: %v", err))
	// 	return
	// }

	// Success
	writeHeader(w, http.StatusOK)
}

type PivotStatus struct {
	PositionDeg float64 `json:"position_deg" db:"position_deg"`
	SpeedPct    float64 `json:"speed_pct" db:"speed_pct"`
	Direction   string  `json:"direction" db:"direction"`
	Wet         bool    `json:"wet" db:"wet"`
	Status      string  `json:"status" db:"status"`
	Battery     float64 `json:"battery_pct" db:"battery_pct"`
}

func getPivotStatus(ctx context.Context, pivotID string) (*PivotStatus, error) {

	// Fetch Status
	var pivotStatus PivotStatus
	err := DB.QueryRow(ctx,
		`SELECT 
			position_deg, 
			speed_pct, 
			direction::text, 
			wet, 
			status::text, 
			battery_pct
        FROM pivot_status
        WHERE pivot_id=$1`,
		pivotID).Scan(
		&pivotStatus.PositionDeg,
		&pivotStatus.SpeedPct,
		&pivotStatus.Direction,
		&pivotStatus.Wet,
		&pivotStatus.Status,
		&pivotStatus.Battery)
	if err != nil {
		return nil, err
	}

	return &pivotStatus, nil
}

func notifyStatusUpdate(ctx context.Context, pivotID string) {

	pivotStatus, err := getPivotStatus(ctx, pivotID)
	if err != nil {
		log.Printf("SSE Broadcast Error: %v", err)
		return
	}

	data, err := json.Marshal(pivotStatus)
	if err != nil {
		return
	}

	subscribersMu.Lock()
	defer subscribersMu.Unlock()

	if pivotSubscribers, ok := subscribers[pivotID]; ok {
		for _, subscriber := range pivotSubscribers {
			select {
			case subscriber.send <- data:
			default:
				//TODO ? Drop if client is too slow to avoid blocking the sync process
			}
		}
	}
}

func handlePivotStatus(w http.ResponseWriter, r *http.Request) {

	pivotID := r.URL.Query().Get("pivot_id")
	if pivotID == "" {
		http.Error(w, "pivot_id required", http.StatusBadRequest)
		return
	}

	// Set SSE Headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	subscriber := &Subscriber{
		pivotID: pivotID,
		send:    make(chan []byte),
	}

	// Register client
	subscribersMu.Lock()
	subscribers[pivotID] = append(subscribers[pivotID], subscriber)
	subscribersMu.Unlock()

	// Unregister client on exit
	defer func() {
		subscribersMu.Lock()
		list := subscribers[pivotID]
		for i, c := range list {
			if c == subscriber {
				subscribers[pivotID] = append(list[:i], list[i+1:]...)
				break
			}
		}
		subscribersMu.Unlock()
		close(subscriber.send)
	}()

	// Initial Response
	pivotStatus, err := getPivotStatus(r.Context(), pivotID)
	if err == nil {
		if data, err := json.Marshal(pivotStatus); err == nil {
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
	} else {
		log.Printf("Initial SSE status fetch failed: %v", err)
	}

	// Listen for data or connection close
	for {
		select {
		case msg := <-subscriber.send:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

type TimerSection struct {
	Serial   int     `json:"serial"`
	SpeedPct float64 `json:"timer_pct"`
	Label    *string `json:"label"`
	Angle    float64 `json:"angle_deg"`
}

func getPivotTimerSections(context context.Context, pivotId string) ([]TimerSection, error) {
	sections := []TimerSection{}

	rows, err := DB.Query(context,
		`SELECT serial, timer_pct, label, angle_deg 
			FROM pivot_timer_sections 
			WHERE pivot_id = $1 
			ORDER BY angle_deg ASC`, pivotId)
	if err != nil {
		return sections, fmt.Errorf("Database error")
	}
	defer rows.Close()

	for rows.Next() {
		var s TimerSection
		if err := rows.Scan(&s.Serial, &s.SpeedPct, &s.Label, &s.Angle); err != nil {
			return sections, fmt.Errorf("Scan error")
		}
		sections = append(sections, s)
	}

	return sections, nil
}

func handleGetPivotTimerSections(w http.ResponseWriter, r *http.Request) {

	// Restrict to GET
	if r.Method != http.MethodGet && r.Method != http.MethodOptions {
		writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var args struct {
		PivotId string `json:"pivot_id"`
	}
	if err := GetArguments(r, &args); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid arguments")
		return
	}

	sections, err := getPivotTimerSections(r.Context(), args.PivotId)
	if err != nil {
		writeText(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sections)

}

func handleRegisterPivotTimerSection(w http.ResponseWriter, r *http.Request) {

	// Handle Preflight
	if r.Method == http.MethodOptions {
		writeHeader(w, http.StatusOK)
		return
	}

	// Restrict to POST
	if r.Method != http.MethodPost {
		writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var args struct {
		PivotId  string   `json:"pivot_id"`
		SpeedPct *float64 `json:"timer_pct"`
		Serial   *int     `json:"serial"` // optional
		Label    *string  `json:"label"`  // optional
		Angle    float64  `json:"angle"`  // optional
	}

	if err := GetArguments(r, &args); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	_, err := DB.Exec(r.Context(), `
		INSERT INTO pivot_timer_sections (pivot_id, serial, timer_pct, label, angle_deg)
			VALUES ($1, $2, COALESCE($3, 100.0), $4, $5)
			ON CONFLICT (pivot_id, serial) 
			DO UPDATE SET 
				timer_pct = COALESCE($3, pivot_timer_sections.timer_pct),
				label     = COALESCE($4, pivot_timer_sections.label),
				angle_deg = $5`,
		args.PivotId, args.Serial, args.SpeedPct, args.Label, args.Angle)

	if err != nil {
		log.Println("ERR: ", err)
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to add section: %v", err))
		return
	}

	sections, err := getPivotTimerSections(r.Context(), args.PivotId)
	if err != nil {
		writeText(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sections)
}

func handleDeletePivotTimerSection(w http.ResponseWriter, r *http.Request) {

	// Handle Preflight
	if r.Method == http.MethodOptions {
		writeHeader(w, http.StatusOK)
		return
	}

	// Restrict to POST
	if r.Method != http.MethodPost && r.Method != http.MethodOptions {
		writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var args struct {
		PivotId string `json:"pivot_id"`
		Serial  *int   `json:"serial"` // optional
	}

	if err := GetArguments(r, &args); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	_, err := DB.Exec(r.Context(), `
		DELETE FROM pivot_timer_sections
		WHERE pivot_id = $1 AND serial = $2`,
		args.PivotId, args.Serial)

	if err != nil {
		writeText(w, http.StatusBadRequest, fmt.Sprintf("Failed to delete section: %v", err))
		return
	}

	sections, err := getPivotTimerSections(r.Context(), args.PivotId)
	if err != nil {
		writeText(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sections)
}

func handleUpdatePivotTimerSection(w http.ResponseWriter, r *http.Request) {

	// Handle Preflight
	if r.Method == http.MethodOptions {
		writeHeader(w, http.StatusOK)
		return
	}

	// Restrict to POST
	if r.Method != http.MethodPost {
		writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var args struct {
		PivotId  string   `json:"pivot_id"`
		Serial   int      `json:"serial"`
		SpeedPct *float64 `json:"timer_pct"` // optional
		Label    *string  `json:"label"`     // optional
		Angle    *float64 `json:"angle_deg"` // optional
	}

	if err := GetArguments(r, &args); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if args.PivotId == "" {
		writeText(w, http.StatusBadRequest, "pivot_id required")
		return
	}

	// Update only provided fields
	_, err := DB.Exec(r.Context(), `
		UPDATE pivot_timer_sections
		SET
			timer_pct = COALESCE($1, timer_pct),
			label     = COALESCE($2, label),
			angle_deg = COALESCE($3, angle_deg)
		WHERE pivot_id = $4 AND serial = $5
	`,
		args.SpeedPct,
		args.Label,
		args.Angle,
		args.PivotId,
		args.Serial,
	)

	if err != nil {
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update section: %v", err))
		return
	}

	// Get Updated Pivot Timer Sections
	sections, err := getPivotTimerSections(r.Context(), args.PivotId)
	if err != nil {
		writeText(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Temporary get string for sections
	payloadBytes, err := json.Marshal(sections)
	if err != nil {
		writeText(w, http.StatusInternalServerError, "Failed to serialize payload")
		return
	}
	payloadStr := string(payloadBytes)

	// Register Update Command
	if err := queuePivotCommand(r.Context(), args.PivotId, "UPDATE", &payloadStr); err != nil {
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to queue command: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, sections)
}

type Command struct {
	ID      int     `json:"id"`
	Command string  `json:"command"`
	Payload *string `json:"payload"`
}

func handleSyncPivot(w http.ResponseWriter, r *http.Request) {

	// Preflight
	if r.Method == http.MethodOptions {
		writeHeader(w, http.StatusOK)
		return
	}

	// Restrict to POST
	if r.Method != http.MethodPost {
		writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Decode body
	var body struct {
		IMEI       string   `json:"imei"`
		Position   *float64 `json:"position_deg"` // optional
		Speed      *float64 `json:"speed_pct"`    // optional
		Direction  *string  `json:"direction"`    // optional
		Wet        *bool    `json:"wet"`          // optional
		Status     *string  `json:"status"`       // optional
		BatteryPct *float64 `json:"battery_pct"`  // optional
	}
	if err := GetArguments(r, &body); err != nil {
		log.Println("err ", err)
		writeText(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.IMEI == "" {
		writeText(w, http.StatusBadRequest, "pivot imei required")
		return
	}

	ctx := r.Context()

	var pivotID string
	err := DB.QueryRow(ctx, `SELECT id FROM pivots WHERE imei = $1`, body.IMEI).Scan(&pivotID)
	if err != nil {
		writeText(w, http.StatusBadRequest, "Invalid IMEI")
		return
	}

	// Begin transaction
	tx, err := DB.Begin(ctx)
	if err != nil {
		writeText(w, http.StatusInternalServerError, "Failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	// Update pivot status
	_, err = tx.Exec(ctx, `
		UPDATE pivot_status
		SET 
			position_deg = COALESCE($1, position_deg),
			speed_pct    = COALESCE($2, speed_pct),
			direction    = COALESCE($3, direction),
			wet          = COALESCE($4, wet),
			status       = COALESCE($5, status),
			battery_pct  = COALESCE($6, battery_pct),
			updated_at   = NOW()
		WHERE pivot_id = $7
	`,
		body.Position,
		body.Speed,
		NormalizeString(body.Direction),
		body.Wet,
		NormalizeString(body.Status),
		body.BatteryPct,
		pivotID,
	)
	if err != nil {
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update status: %v", err))
		return
	}

	// Fetch unacknowledged commands
	rows, err := tx.Query(ctx, `
		SELECT id, command, payload
		FROM pivot_command_queue
		WHERE pivot_id = $1 AND acknowledged = FALSE
		ORDER BY created_at ASC
		FOR UPDATE
	`, pivotID)
	if err != nil {
		writeText(w, http.StatusInternalServerError, "Failed to fetch commands")
		return
	}
	defer rows.Close()

	var commands []Command
	var ids []int

	for rows.Next() {
		var c Command
		if err := rows.Scan(&c.ID, &c.Command, &c.Payload); err != nil {
			writeText(w, http.StatusInternalServerError, "Scan error")
			return
		}
		commands = append(commands, c)
		ids = append(ids, c.ID)
	}

	// Acknowledge commands
	if len(ids) > 0 {
		_, err = tx.Exec(ctx, `
			UPDATE pivot_command_queue
			SET 
				acknowledged = TRUE,
				acknowledged_at = NOW()
			WHERE id = ANY($1)
		`, ids)

		if err != nil {
			writeText(w, http.StatusInternalServerError, "Failed to acknowledge commands")
			return
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		writeText(w, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	go notifyStatusUpdate(context.Background(), pivotID)

	writeJSON(w, http.StatusOK, commands)
}
