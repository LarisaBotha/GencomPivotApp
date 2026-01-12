package main

import (
	"fmt"
	"log"
	"net/http"
)

var Commands = []string{"Start", "Stop"}

func main() {
	InitDB()
	defer CloseDB()

	http.HandleFunc("/api/ping", handlePing)
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/register_user", handleRegisterUser)
	http.HandleFunc("/api/register_pivot", handleRegisterPivot)
	http.HandleFunc("/api/get_user_pivots", handleGetUserPivots)
	http.HandleFunc("/api/command", handleCommand)

	fmt.Println("🚀 Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
