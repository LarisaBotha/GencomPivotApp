package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

var Commands = []string{"Start", "Stop", "Update"}
var Direction = []string{"forward", "reverse"}
var Status = []string{"stopped", "running", "offline", "error"}

type Subscriber struct {
	pivotID string
	send    chan []byte
}

var (
	subscribers   = make(map[string][]*Subscriber)
	subscribersMu sync.Mutex
)

func main() {
	InitDB()
	defer CloseDB()

	http.HandleFunc("/api/ping", handlePing)
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/register_user", handleRegisterUser)
	http.HandleFunc("/api/register_pivot", handleRegisterPivot)
	http.HandleFunc("/api/get_user_pivots", handleGetUserPivots)
	http.HandleFunc("/api/pivot_status", handlePivotStatus)
	http.HandleFunc("/api/get_pivot_timer_sections", handleGetPivotTimerSections)
	http.HandleFunc("/api/register_pivot_timer_section", handleRegisterPivotTimerSection)
	http.HandleFunc("/api/delete_pivot_timer_section", handleDeletePivotTimerSection)
	http.HandleFunc("/api/update_pivot_timer_section", handleUpdatePivotTimerSection)
	http.HandleFunc("/api/sync_pivot", handleSyncPivot)
	http.HandleFunc("/api/command", handleCommand)

	fmt.Println("🚀 Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
