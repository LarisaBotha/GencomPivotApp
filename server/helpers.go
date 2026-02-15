package main

import (
	"encoding/json"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

func writeHeader(w http.ResponseWriter, status int) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
}

func writeResponse(w http.ResponseWriter, status int, contentType string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Connection", "close")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))

	writeHeader(w, status)
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		writeText(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	writeResponse(w, status, "application/json", b)
}

func writeText(w http.ResponseWriter, status int, message string) {
	writeResponse(w, status, "text/plain; charset=utf-8", []byte(message))
}

func GetArguments(r *http.Request, dst any) error {
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		return json.NewDecoder(r.Body).Decode(dst)
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	v := reflect.ValueOf(dst).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		key := field.Tag.Get("json")
		if key == "" || key == "_" {
			key = strings.ToLower(field.Name)
		}

		var val string
		if r.Method == http.MethodGet {
			val = r.URL.Query().Get(key)
		} else {
			val = r.PostFormValue(key)
		}

		if val != "" {
			v.Field(i).SetString(val)
		}
	}
	return nil
}
