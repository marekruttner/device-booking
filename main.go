package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"time"

	_ "github.com/lib/pq"
)

type User struct {
	ID       int
	Username string
	Password string
}

type Device struct {
	ID   int
	Name string
}

var (
	db      *sql.DB
	devices []Device
)

func main() {
	// Initialize the database connection
	initDB()

	// Initialize some sample devices
	initializeDevices()

	http.HandleFunc("/", calendarHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/admin", adminHandler)
	http.HandleFunc("/admin/adduser", addUserHandler)
	http.HandleFunc("/admin/adddevice", addDeviceHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func initDB() {
	// Open a database connection
	var err error
	db, err = sql.Open("postgres", "postgres://postgres:NvsWrkD3V@localhost/device-booking_db?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	// Create the users table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username TEXT NOT NULL,
			password TEXT NOT NULL,
			is_first_login BOOLEAN NOT NULL DEFAULT true
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// Check if the default admin user exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'admin'").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	// If the default admin user doesn't exist, create it
	if count == 0 {
		_, err = db.Exec("INSERT INTO users (username, password) VALUES ('admin', 'password')")
		if err != nil {
			log.Fatal(err)
		}
	}

	// ... Rest of the code
}

func initializeDevices() {
	// Fetch devices from the database
	rows, err := db.Query("SELECT id, name FROM devices")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var device Device
		err := rows.Scan(&device.ID, &device.Name)
		if err != nil {
			log.Fatal(err)
		}
		devices = append(devices, device)
	}
}

func calendarHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the user is authenticated
	if !isAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get the current date
	now := time.Now()

	// Generate calendar data
	calendarData := generateCalendarData(now.Year(), now.Month())

	// Render the template with calendar data
	renderTemplate(w, "calendar.html", calendarData)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		renderTemplate(w, "login.html", nil)
	} else if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		// Check if the provided credentials are valid
		if isValidUser(username, password) {
			setAuthenticated(w, username)
			http.Redirect(w, r, "/", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}
	}
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the user is authenticated as an admin
	if !isAuthenticated(r) || !isAdmin(r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Render the admin template
	renderTemplate(w, "admin.html", devices)
}

func addUserHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the user is authenticated as an admin
	if !isAuthenticated(r) || !isAdmin(r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "GET" {
		renderTemplate(w, "add_user.html", nil)
	} else if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		// Insert the new user into the database
		_, err := db.Exec("INSERT INTO users (username, password) VALUES ($1, $2)", username, password)
		if err != nil {
			log.Fatal(err)
		}

		// Redirect back to the admin panel
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

func addDeviceHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the user is authenticated as an admin
	if !isAuthenticated(r) || !isAdmin(r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "GET" {
		renderTemplate(w, "add_device.html", nil)
	} else if r.Method == "POST" {
		deviceName := r.FormValue("devicename")

		// Insert the new device into the database
		_, err := db.Exec("INSERT INTO devices (name) VALUES ($1)", deviceName)
		if err != nil {
			log.Fatal(err)
		}

		// Reload devices from the database
		initializeDevices()

		// Redirect back to the admin panel
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

func generateCalendarData(year int, month time.Month) []int {
	// Get the first day of the month
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	// Get the number of days in the month
	numDays := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()

	// Generate a slice with the days of the month
	calendarData := make([]int, numDays)

	for i := 0; i < numDays; i++ {
		calendarData[i] = firstDay.Day() + i
	}

	return calendarData
}

func renderTemplate(w http.ResponseWriter, templateFile string, data interface{}) {
	tmpl, err := template.ParseFiles(templateFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func isAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie("authenticated")
	if err != nil || cookie.Value == "" {
		return false
	}
	return true
}

func isAdmin(r *http.Request) bool {
	cookie, err := r.Cookie("authenticated")
	if err != nil || cookie.Value != "admin" {
		return false
	}
	return true
}

func setAuthenticated(w http.ResponseWriter, username string) {
	expiration := time.Now().Add(24 * time.Hour)
	cookie := http.Cookie{Name: "authenticated", Value: username, Expires: expiration}
	http.SetCookie(w, &cookie)
}

func isValidUser(username, password string) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = $1 AND password = $2", username, password).Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	return count > 0
}
