package main

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

func writeHeader(w http.ResponseWriter, status int) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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

func NormalizeString(s *string) *string {
	if s == nil {
		return nil
	}
	str := strings.ToLower(strings.TrimSpace(*s))
	return &str
}

// func GetArguments(r *http.Request, dst any) error {
// 	contentType := r.Header.Get("Content-Type")

// 	// 1. Handle JSON (Primary for modern APIs)
// 	if strings.Contains(contentType, "application/json") {
// 		// TEMPORARY DEBUG:
// 		bodyBytes, _ := io.ReadAll(r.Body)
// 		log.Println("RAW BODY:", string(bodyBytes))
// 		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Reset body for decoder

// 		return json.NewDecoder(r.Body).Decode(dst)
// 	}

// 	// 2. Handle Form/URL Params (Fallback)
// 	if err := r.ParseForm(); err != nil {
// 		return err
// 	}

// 	v := reflect.ValueOf(dst).Elem()
// 	t := v.Type()

// 	for i := 0; i < t.NumField(); i++ {
// 		field := t.Field(i)
// 		fieldValue := v.Field(i)

// 		// Get the key from the json tag or lowercase field name
// 		key := field.Tag.Get("json")
// 		if key == "" || key == "-" {
// 			key = strings.ToLower(field.Name)
// 		}

// 		// Get value from Query (GET) or PostForm (POST/PUT)
// 		val := r.FormValue(key)
// 		if val == "" {
// 			continue
// 		}

// 		// 3. Type-Safe Assignment using Reflection
// 		switch fieldValue.Kind() {
// 		case reflect.String:
// 			fieldValue.SetString(val)
// 		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
// 			if i, err := strconv.ParseInt(val, 10, 64); err == nil {
// 				fieldValue.SetInt(i)
// 			}
// 		case reflect.Float32, reflect.Float64:
// 			if f, err := strconv.ParseFloat(val, 64); err == nil {
// 				fieldValue.SetFloat(f)
// 			}
// 		case reflect.Bool:
// 			if b, err := strconv.ParseBool(val); err == nil {
// 				fieldValue.SetBool(b)
// 			}
// 		case reflect.Pointer:
// 			// Handling pointers (like *int or *float64) is more complex.
// 			// For simplicity in this handler, we target the base types.
// 			// If you use pointers extensively, you'd need to Initialize the pointer here.
// 		}
// 	}
// 	return nil
// }
