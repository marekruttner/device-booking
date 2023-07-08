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

var users = []User{
	{Username: "admin", Password: "password"},
	{Username: "user", Password: "123456"},
}

func main() {
	http.HandleFunc("/", calendarHandler)
	http.HandleFunc("/login", loginHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
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
