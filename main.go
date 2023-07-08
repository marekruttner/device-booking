package main

import (
	"html/template"
	"log"
	"net/http"
	"time"
)

type User struct {
	Username string
	Password string
}

type Device struct {
	ID   int
	Name string
}

var (
	users   []User
	devices []Device
)

func main() {
	// Initialize some sample users and devices
	initializeData()

	http.HandleFunc("/", calendarHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/admin", adminHandler)
	http.HandleFunc("/admin/adduser", addUserHandler)
	http.HandleFunc("/admin/adddevice", addDeviceHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func initializeData() {
	// Sample users
	users = []User{
		{Username: "admin", Password: "password"},
		{Username: "user", Password: "123456"},
	}

	// Sample devices
	devices = []Device{
		{ID: 1, Name: "Device 1"},
		{ID: 2, Name: "Device 2"},
		{ID: 3, Name: "Device 3"},
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

		// Add the new user
		newUser := User{Username: username, Password: password}
		users = append(users, newUser)

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

		// Generate a unique ID for the new device
		newDeviceID := len(devices) + 1

		// Add the new device
		newDevice := Device{ID: newDeviceID, Name: deviceName}
		devices = append(devices, newDevice)

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
	for _, user := range users {
		if user.Username == username && user.Password == password {
			return true
		}
	}
	return false
}
