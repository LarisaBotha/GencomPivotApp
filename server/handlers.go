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
	type dbPivot struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var pivots []dbPivot

	for rows.Next() {
		var p dbPivot
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			writeText(w, http.StatusInternalServerError, fmt.Sprintf("Internal server error: %v", err))
			return
		}
		pivots = append(pivots, p)
	}

	writeJSON(w, http.StatusOK, pivots)
}

func queuePivotCommand(ctx context.Context, pivotID string, cmd string, payload *string) error {

	if !slices.Contains(Commands, cmd) {
		return fmt.Errorf("unknown command: %s", cmd)
	}

	_, err := DB.Exec(ctx, `
        INSERT INTO pivot_command_queue (pivot_id, command, payload) 
        VALUES ($1, $2, $3);`,
		pivotID, cmd, payload)
	return err
}

type dbPivotStatus struct {
	PositionDeg float64 `json:"position_deg" db:"position_deg"`
	SpeedPct    float64 `json:"speed_pct" db:"speed_pct"`
	Direction   string  `json:"direction" db:"direction"`
	Wet         bool    `json:"wet" db:"wet"`
	Status      string  `json:"status" db:"status"`
	Battery     float64 `json:"battery_pct" db:"battery_pct"`
	Pressure    float64 `json:"pressure" db:"pressure"`
}

func getPivotStatus(ctx context.Context, pivotID string) (*dbPivotStatus, error) {

	// Fetch Status
	var pivotStatus dbPivotStatus
	err := DB.QueryRow(ctx,
		`SELECT 
			position_deg, 
			speed_pct, 
			direction::text, 
			wet, 
			status::text, 
			battery_pct,
			pressure
        FROM pivot_status
        WHERE pivot_id=$1`,
		pivotID).Scan(
		&pivotStatus.PositionDeg,
		&pivotStatus.SpeedPct,
		&pivotStatus.Direction,
		&pivotStatus.Wet,
		&pivotStatus.Status,
		&pivotStatus.Battery,
		&pivotStatus.Pressure,
	)
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

type dbPivotSection struct {
	Serial int     `json:"serial"`
	Value  float64 `json:"value"`
	Label  *string `json:"label"`
	Angle  float64 `json:"angle_deg"`
	Unit   string  `json:"unit"`
}

func getPivotSections(context context.Context, pivotId string) ([]dbPivotSection, error) {
	sections := []dbPivotSection{}

	rows, err := DB.Query(context,
		`SELECT serial, value, label, angle_deg, unit
			FROM pivot_sections 
			WHERE pivot_id = $1 
			ORDER BY angle_deg ASC`, pivotId)
	if err != nil {
		return sections, fmt.Errorf("Database error")
	}
	defer rows.Close()

	for rows.Next() {
		var s dbPivotSection
		if err := rows.Scan(&s.Serial, &s.Value, &s.Label, &s.Angle, &s.Unit); err != nil {
			return sections, fmt.Errorf("Scan error")
		}
		sections = append(sections, s)
	}

	return sections, nil
}

func handleGetPivotSections(w http.ResponseWriter, r *http.Request) {

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

	sections, err := getPivotSections(r.Context(), args.PivotId)
	if err != nil {
		writeText(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sections)

}

func handleRegisterPivotSection(w http.ResponseWriter, r *http.Request) {

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
		PivotId string   `json:"pivot_id"`
		Value   *float64 `json:"value"`
		Serial  *int     `json:"serial"` // optional
		Label   *string  `json:"label"`  // optional
		Angle   float64  `json:"angle"`  // optional
	}

	if err := GetArguments(r, &args); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	_, err := DB.Exec(r.Context(), `
		INSERT INTO pivot_sections (pivot_id, serial, value, label, angle_deg)
			VALUES ($1, $2, COALESCE($3, 100.0), $4, $5)
			ON CONFLICT (pivot_id, serial) 
			DO UPDATE SET 
				value	  = COALESCE($3, pivot_sections.value),
				label     = COALESCE($4, pivot_sections.label),
				angle_deg = $5`,
		args.PivotId, args.Serial, args.Value, args.Label, args.Angle)

	if err != nil {
		log.Println("ERR: ", err)
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to add section: %v", err))
		return
	}

	sections, err := getPivotSections(r.Context(), args.PivotId)
	if err != nil {
		writeText(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sections)
}

func handleDeletePivotSection(w http.ResponseWriter, r *http.Request) {

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
		DELETE FROM pivot_sections
		WHERE pivot_id = $1 AND serial = $2`,
		args.PivotId, args.Serial)

	if err != nil {
		writeText(w, http.StatusBadRequest, fmt.Sprintf("Failed to delete section: %v", err))
		return
	}

	sections, err := getPivotSections(r.Context(), args.PivotId)
	if err != nil {
		writeText(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sections)
}

type SectionsSimplified struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

func handleUpdatePivotSection(w http.ResponseWriter, r *http.Request) {

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
		PivotId string   `json:"pivot_id"`
		Serial  int      `json:"serial"`
		Value   *float64 `json:"value"`     // optional
		Label   *string  `json:"label"`     // optional
		Angle   *float64 `json:"angle_deg"` // optional
		Unit    *string  `json:"unit"`      // optional
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
		UPDATE pivot_sections
		SET
			value 	  = COALESCE($1, value),
			label     = COALESCE($2, label),
			angle_deg = COALESCE($3, angle_deg),
			unit 	  = COALESCE($4, unit)
		WHERE pivot_id = $5 AND serial = $6
	`,
		args.Value,
		args.Label,
		args.Angle,
		args.Unit,
		args.PivotId,
		args.Serial,
	)

	if err != nil {
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update section: %v", err))
		return
	}

	// Get Updated Pivot Sections
	sections, err := getPivotSections(r.Context(), args.PivotId)
	if err != nil {
		writeText(w, http.StatusInternalServerError, err.Error())
		return
	}

	var simplifiedSections []SectionsSimplified

	for i := range sections {
		start := sections[i].Angle
		var end float64

		if i+1 < len(sections) {
			end = sections[i+1].Angle
		} else {
			end = sections[0].Angle
		}

		simplifiedSections = append(simplifiedSections, SectionsSimplified{
			Start: start,
			End:   end,
			Value: sections[i].Value,
			Unit:  sections[i].Unit,
		})
	}

	// Serialize simplified sections
	payloadBytes, err := json.Marshal(simplifiedSections)
	if err != nil {
		writeText(w, http.StatusInternalServerError, "Failed to serialize payload")
		return
	}
	payloadStr := string(payloadBytes)

	// Register Update Command
	if err := queuePivotCommand(r.Context(), args.PivotId, "Update", &payloadStr); err != nil {
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to queue command: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, sections)
}

func getPivotIDByIMEI(ctx context.Context, imei string) (string, error) {
	var id string
	err := DB.QueryRow(ctx, `SELECT id FROM pivots WHERE imei = $1`, imei).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

type OrderedSection struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

func (os OrderedSection) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(
		`{"start":%g,"end":%g,"value":%g,"unit":%q}`,
		os.Start, os.End, os.Value, os.Unit,
	)), nil
}

type CommandBlock struct {
	ID      int
	Payload *string
}

// MarshalJSON guarantees "id" is first, and fields inside sections arrays are ordered uniformly
func (cb CommandBlock) MarshalJSON() ([]byte, error) {
	idPart := fmt.Sprintf(`{"id":%d`, cb.ID)

	// If there's no payload, cleanly close the object
	if cb.Payload == nil || *cb.Payload == "" {
		return []byte(idPart + "}"), nil
	}

	p := *cb.Payload

	// If the payload is a JSON array (like sections), deserialize and enforce strict order
	if p[0] == '[' {
		var rawSections []OrderedSection
		if err := json.Unmarshal([]byte(p), &rawSections); err == nil {
			orderedBytes, _ := json.Marshal(rawSections)
			return []byte(fmt.Sprintf(`%s,"sections":%s}`, idPart, string(orderedBytes))), nil
		}
	}

	// If the payload is a standard JSON object, strip its opening '{' and merge it right next to id
	if p[0] == '{' {
		return []byte(idPart + "," + p[1:]), nil
	}

	return []byte(idPart + "}"), nil
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
		IMEI       string                `json:"imei"`
		Position   *float64              `json:"position_deg"` // optional
		Speed      *float64              `json:"speed_pct"`    // optional
		Direction  *string               `json:"direction"`    // optional
		Wet        *bool                 `json:"wet"`          // optional
		Status     *string               `json:"status"`       // optional
		BatteryPct *float64              `json:"battery_pct"`  // optional
		Sections   *[]SectionsSimplified `json:"sections"`     // optional
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

	pivotID, err := getPivotIDByIMEI(ctx, body.IMEI)
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

	// Ensure Sections still align
	if body.Sections != nil {
		sections := *body.Sections

		// Sort the sections by their starting angle (0° -> 360°)
		slices.SortFunc(sections, func(a, b SectionsSimplified) int {
			if a.Start < b.Start {
				return -1
			}
			if a.Start > b.Start {
				return 1
			}
			return 0
		})

		for _, s := range sections {
			_, err = tx.Exec(ctx, `
            UPDATE pivot_sections
            SET 
                value = $1,
                unit  = $2
            WHERE pivot_id = $3 AND angle_deg = $4
        `, s.Value, s.Unit, pivotID, s.Start)

			if err != nil {
				writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to sync section at angle %v: %v", s.Start, err))
				return
			}
		}
	}

	// Fetch unacknowledged commands
	rows, err := tx.Query(ctx, `
            WITH latest_commands AS (
            SELECT DISTINCT ON (command) id
            FROM pivot_command_queue
            WHERE pivot_id = $1 AND acknowledged = FALSE
            ORDER BY command, created_at DESC
        )
        SELECT q.id, q.command, q.payload
        FROM pivot_command_queue q
        JOIN latest_commands lc ON q.id = lc.id
        FOR UPDATE
    `, pivotID)
	if err != nil {
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch commands: %v", err))
		return
	}
	defer rows.Close()

	response := make(map[string]any)

	subscribersMu.Lock()
	if list, ok := subscribers[pivotID]; ok {
		response["connections"] = len(list)
	} else {
		response["connections"] = 0
	}
	subscribersMu.Unlock()

	for rows.Next() {
		var id int
		var command string
		var payload *string

		if err := rows.Scan(&id, &command, &payload); err != nil {
			writeText(w, http.StatusInternalServerError, fmt.Sprintf("Scan error: %v", err))
			return
		}

		jsonKey := *NormalizeString(&command)

		// Assigning our CommandBlock automatically wires the strict serialization output logic
		response[jsonKey] = CommandBlock{
			ID:      id,
			Payload: payload,
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to commit transaction: %v", err))
		return
	}

	go notifyStatusUpdate(context.Background(), pivotID)

	writeJSON(w, http.StatusOK, response)
}

func handleGetSubscriberCount(w http.ResponseWriter, r *http.Request) {

	// Restrict to GET
	if r.Method != http.MethodGet && r.Method != http.MethodOptions {
		writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get arguments
	var args struct {
		IMEI string `json:"imei"`
	}
	if err := GetArguments(r, &args); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid arguments")
		return
	}

	// Find Pivot Id by imei
	pivotID, err := getPivotIDByIMEI(r.Context(), args.IMEI)
	if err != nil {
		writeText(w, http.StatusBadRequest, "Invalid IMEI")
		return
	}

	subscribersMu.Lock()
	count := 0
	if list, ok := subscribers[pivotID]; ok {
		count = len(list)
	}
	subscribersMu.Unlock()

	// Return the count
	writeJSON(w, http.StatusOK, map[string]int{
		"count": count,
	})
}

func handleUpdatePivotControl(w http.ResponseWriter, r *http.Request) {

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

	// Decode Body
	var body struct {
		PivotId   string  `json:"pivot_id"`
		Direction *string `json:"direction"`
		Wet       *bool   `json:"wet"`
	}

	if err := GetArguments(r, &body); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.PivotId == "" {
		writeText(w, http.StatusBadRequest, "pivot_id is required")
		return
	}

	if body.Direction != nil && !slices.Contains(Direction, *body.Direction) {
		writeText(w, http.StatusBadRequest, "Invalid direction")
	}

	// Update
	var finalDirection string
	var finalWet bool
	err := DB.QueryRow(r.Context(), `
		UPDATE pivot_status 
		SET 
			direction = COALESCE($1, direction),
			wet = COALESCE($2, wet),
			updated_at = NOW()
		WHERE pivot_id = $3
		RETURNING direction, wet`,
		NormalizeString(body.Direction),
		body.Wet,
		body.PivotId,
	).Scan(&finalDirection, &finalWet)

	if err != nil {
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update status: %v", err))
		return
	}

	writeHeader(w, http.StatusOK)
}

func handleAckCommands(w http.ResponseWriter, r *http.Request) {

	// Restrict to POST
	if r.Method != http.MethodPost && r.Method != http.MethodOptions {
		writeText(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get arguments
	var body struct {
		IDs []int `json:"ids"`
	}
	if err := GetArguments(r, &body); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid arguments")
		return
	}

	// Acknowledge commands
	if len(body.IDs) > 0 {
		_, err := DB.Exec(r.Context(), `
			UPDATE pivot_command_queue
			SET
				acknowledged = TRUE,
				acknowledged_at = NOW()
			WHERE id = ANY($1)
		`, body.IDs)

		if err != nil {
			writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to acknowledge commands: %v", err))
			return
		}
	}

	writeHeader(w, http.StatusOK)
}

func handleStart(w http.ResponseWriter, r *http.Request) {

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

	// Decode Body
	var body struct {
		PivotId string `json:"pivot_id"`
	}
	if err := GetArguments(r, &body); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.PivotId == "" {
		writeText(w, http.StatusBadRequest, "pivot_id is required")
		return
	}

	ctx := r.Context()

	// Fetch current pivot controls (wet and direction status)
	var direction string
	var wet bool
	err := DB.QueryRow(ctx, `
		SELECT direction::text, wet 
		FROM pivot_status 
		WHERE pivot_id = $1
	`, body.PivotId).Scan(&direction, &wet)

	if err != nil {
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch pivot control settings: %v", err))
		return
	}

	// Queue command
	payload := map[string]any{
		"direction": direction,
		"wet":       wet,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		writeText(w, http.StatusInternalServerError, "Failed to serialize payload")
		return
	}
	payloadStr := string(payloadBytes)
	if err := queuePivotCommand(ctx, body.PivotId, "Start", &payloadStr); err != nil {
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to queue start command: %v", err))
		return
	}

	writeHeader(w, http.StatusOK)
}

func handleStop(w http.ResponseWriter, r *http.Request) {

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

	// Decode Body
	var body struct {
		PivotId string `json:"pivot_id"`
	}
	if err := GetArguments(r, &body); err != nil {
		writeText(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validation
	if body.PivotId == "" {
		writeText(w, http.StatusBadRequest, "pivot_id is required")
		return
	}

	// Queue command
	if err := queuePivotCommand(r.Context(), body.PivotId, "Stop", nil); err != nil {
		writeText(w, http.StatusInternalServerError, fmt.Sprintf("Failed to queue stop command: %v", err))
		return
	}

	writeHeader(w, http.StatusOK)
}
