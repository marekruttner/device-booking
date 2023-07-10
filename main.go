package main

import (
	"database/sql"
	"encoding/csv"
	"github.com/gorilla/mux"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

type User struct {
	ID       int
	Username string
	Password string
}

type Device struct {
	ID         int
	Name       string
	InternalID string
	StartDate  time.Time
	EndDate    time.Time
	Bookings   []Booking
}

type CalendarData struct {
	IsAdmin bool
	Days    []int
}

type DayData struct {
	Day          int
	DeviceBooked map[int]Booking
	Days         []int
	BookedBy     int
	BookingTime  map[int]time.Time
}

type Booking struct {
	ID        int
	DeviceID  int
	UserID    int
	StartDate time.Time
	EndDate   time.Time
}

type DeviceBooking struct {
	DeviceID  int
	BookedBy  int
	StartDate time.Time
	EndDate   time.Time
}

type MenuData struct {
	IsAdmin  bool
	Devices  []Device
	Bookings []Booking
}

var (
	db            *sql.DB
	devices       []Device
	calendarData  []DayData
	bookings      []Booking
	nextBookingID int
)

func main() {
	// Initialize the database connection
	initDB()

	// Initialize some sample devices
	initializeDevices()

	router := mux.NewRouter()

	// Define the routes
	router.HandleFunc("/", calendarHandler)
	router.HandleFunc("/login", loginHandler)
	router.HandleFunc("/admin", adminHandler)
	router.HandleFunc("/admin/adduser", addUserHandler)
	router.HandleFunc("/admin/adddevice", addDeviceHandler)
	router.HandleFunc("/admin/bookdevice", bookDeviceHandler)
	router.HandleFunc("/admin/import", importHandler) // Use POST method for importHandler

	log.Fatal(http.ListenAndServe(":8080", router))
}
func initDB() {
	// Open a database connection
	var err error
	db, err = sql.Open("postgres", "postgres://postgres:NvsWrkD3V@localhost:5432/device-booking_db?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	// Create the bookings table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bookings (
			id SERIAL PRIMARY KEY,
			device_id INT NOT NULL,
			user_id INT NOT NULL,
			start_date TIMESTAMPTZ,
			end_date TIMESTAMPTZ
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
}

func initializeDevices() {
	// Fetch devices from the database
	rows, err := db.Query("SELECT id, name, internalid FROM devices")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	devices = []Device{} // Clear the devices slice

	now := time.Now() // Get the current time

	for rows.Next() {
		var device Device
		err := rows.Scan(&device.ID, &device.Name, &device.InternalID)
		if err != nil {
			log.Fatal(err)
		}

		// Fetch bookings for the device from the database
		bookings, err := getDeviceBookings(device.ID, time.Month(now.Month()), now.Year())
		if err != nil {
			log.Fatal(err)
		}

		device.Bookings = bookings // Assign fetched bookings to the device

		devices = append(devices, device)
	}

	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}

func initializeBookings() {
	// Fetch bookings from the database
	rows, err := db.Query("SELECT id, device_id, user_id, start_date, end_date FROM bookings")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	bookings = []Booking{} // Clear the bookings slice

	for rows.Next() {
		var booking Booking
		err := rows.Scan(&booking.ID, &booking.DeviceID, &booking.UserID, &booking.StartDate, &booking.EndDate)
		if err != nil {
			log.Fatal(err)
		}
		bookings = append(bookings, booking)
	}

	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}

func calendarHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve devices from the database
	initializeDevices()

	// Retrieve bookings from the database
	initializeBookings()

	// Get the current month and year
	now := time.Now()
	year, month, _ := now.Date()

	// Generate calendar data for the current month
	calendarData, err := generateCalendarData(year, month)
	if err != nil {
		log.Fatal(err)
	}

	// Render the calendar template with device and booking information
	data := struct {
		IsAdmin      bool
		CalendarData []DayData
	}{
		IsAdmin:      isAdmin(r),
		CalendarData: calendarData,
	}
	renderTemplate(w, "calendar.html", data)
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

	// Handle the import form submission
	if r.Method == "POST" && r.URL.Path == "/admin/import" {
		// Parse the uploaded CSV file
		file, _, err := r.FormFile("csvfile")
		if err != nil {
			http.Error(w, "Failed to parse CSV file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Read the CSV data
		reader := csv.NewReader(file)
		records, err := reader.ReadAll()
		if err != nil {
			http.Error(w, "Failed to read CSV data", http.StatusInternalServerError)
			return
		}

		// Process the device records from the CSV file
		for _, record := range records {
			// Extract the device ID and name from the CSV record
			deviceID := record[0]
			deviceName := record[1]

			// Modify the device ID as desired (e.g., add a prefix)
			//modifiedDeviceID := "23A" + deviceID

			// Insert the modified device into the database
			_, err = db.Exec("INSERT INTO devices (id, name) VALUES ($1, $2)", deviceID, deviceName)
			if err != nil {
				http.Error(w, "Failed to insert device into the database", http.StatusInternalServerError)
				return
			}
		}

		// Redirect back to the admin panel
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	// Handle the manual device addition form submission
	if r.Method == "POST" && r.URL.Path == "/admin/adddevice" {
		deviceName := r.FormValue("devicename")

		// Insert the new device into the database
		_, err := db.Exec("INSERT INTO devices (name) VALUES ($1)", deviceName)
		if err != nil {
			http.Error(w, "Failed to insert device into the database", http.StatusInternalServerError)
			return
		}

		// Redirect back to the admin panel
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	// Update the data structure for the menu template
	menuData := struct {
		IsAdmin  bool
		Devices  []Device
		Bookings []Booking
	}{
		IsAdmin:  isAdmin(r),
		Devices:  devices,
		Bookings: bookings,
	}

	// Render the admin template with menu data
	renderTemplate(w, "admin.html", menuData)
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

func bookDeviceHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the user is authenticated as an admin
	if !isAuthenticated(r) || !isAdmin(r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Retrieve the device ID, start date, and end date from the form submission
	deviceIDStr := r.FormValue("deviceid")
	startDateStr := r.FormValue("startdate")
	endDateStr := r.FormValue("enddate")

	// Convert the device ID to an integer
	deviceID, err := strconv.Atoi(deviceIDStr)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Convert the start and end dates to time.Time objects
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		http.Error(w, "Invalid start date", http.StatusBadRequest)
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		http.Error(w, "Invalid end date", http.StatusBadRequest)
		return
	}

	// Find the device in the devices slice by ID
	device := findDeviceByID(strconv.Itoa(deviceID))
	if device == nil {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	// Get the user ID of the logged-in user
	userID := getUserID(r)

	// Perform validation checks
	if !isBookingValid(startDate, endDate, device.ID) {
		http.Error(w, "Invalid booking", http.StatusBadRequest)
		return
	}

	// Update the device's start and end dates
	device.StartDate = startDate
	device.EndDate = endDate

	// Insert the booking into the database
	_, err = db.Exec("INSERT INTO bookings (device_id, user_id, start_date, end_date) VALUES ($1, $2, $3, $4)",
		deviceID, userID, startDate, endDate)
	if err != nil {
		http.Error(w, "Failed to create booking", http.StatusInternalServerError)
		return
	}

	// Redirect back to the admin panel
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func importHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the multipart form data from the request
	err := r.ParseMultipartForm(10 << 20) // Set the maximum file size to 10MB
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusInternalServerError)
		return
	}

	// Retrieve the uploaded CSV file from the form data
	file, _, err := r.FormFile("csvfile")
	if err != nil {
		http.Error(w, "Failed to retrieve CSV file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Read the CSV file using a CSV reader
	reader := csv.NewReader(file)

	// Read all the records from the CSV file
	records, err := reader.ReadAll()
	if err != nil {
		http.Error(w, "Failed to read CSV data", http.StatusInternalServerError)
		return
	}

	// Process the CSV records
	for _, record := range records {
		deviceID := record[0]
		deviceName := record[1]

		// Modify the device ID as desired (e.g., add a prefix)
		//modifiedDeviceID := deviceID

		// Insert the modified device into the database
		_, err = db.Exec("INSERT INTO devices (id, name) VALUES ($1, $2)", deviceID, deviceName)
		if err != nil {
			http.Error(w, "Failed to insert device into the database", http.StatusInternalServerError)
			return
		}
	}

	// Redirect back to the admin panel
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func generateCalendarData(year int, month time.Month) ([]DayData, error) {
	// Get the first day of the month
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	// Get the number of days in the month
	numDays := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()

	// Generate a slice with the days of the month
	calendarData := make([]DayData, numDays)

	for i := 0; i < numDays; i++ {
		day := firstDay.Day() + i

		// Fetch the device bookings for the day from the database
		bookings, err := getDeviceBookings(day, month, year)
		if err != nil {
			return nil, err
		}

		// Create a map to store the device bookings
		deviceBookings := make(map[int]Booking)
		for _, booking := range bookings {
			deviceBookings[booking.DeviceID] = booking
		}

		calendarData[i] = DayData{
			Day:          day,
			DeviceBooked: deviceBookings,
		}
	}

	return calendarData, nil
}

func monthYearString(year int, month time.Month) string {
	date := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	return date.Format("January 2006")
}

func isDeviceBooked(day int, month time.Month, year int, deviceID int) bool {
	for _, booking := range bookings {
		if booking.DeviceID == deviceID && booking.StartDate.Day() <= day && booking.EndDate.Day() >= day &&
			booking.StartDate.Month() == month && booking.EndDate.Month() == month &&
			booking.StartDate.Year() == year && booking.EndDate.Year() == year {
			return true
		}
	}

	return false
}

/*
*

	func updateCalendarData(startDate, endDate time.Time, deviceID, userID int) {
		// Loop through the calendar data and update the booked devices for the specified dates
		for i := range calendarData {
			if calendarData[i].Day >= startDate.Day() && calendarData[i].Day <= endDate.Day() {
				if calendarData[i].DeviceBooked == nil {
					calendarData[i].DeviceBooked = make(map[int]bool)
				}
				calendarData[i].DeviceBooked[deviceID] = true
				calendarData[i].BookedBy = userID
			}
		}
	}

*
*/

func isBookingValid(startDate, endDate time.Time, deviceID int) bool {
	// Check if the start date is before the end date
	if startDate.After(endDate) || startDate.Equal(endDate) {
		return false
	}

	// Check if the device is already booked during the selected dates
	for _, booking := range bookings {
		// Skip the current booking for the same device
		if booking.DeviceID == deviceID {
			continue
		}

		// Check for overlapping dates
		if startDate.Before(booking.EndDate) && endDate.After(booking.StartDate) {
			return false
		}
	}

	return true
}

func findDeviceByID(deviceID string) *Device {
	id, err := strconv.Atoi(deviceID)
	if err != nil {
		return nil
	}

	for i := range devices {
		if devices[i].ID == id {
			return &devices[i]
		}
	}

	return nil
}

func getDeviceBookings(deviceID int, month time.Month, year int) ([]Booking, error) {
	bookings := []Booking{}

	// Fetch bookings for the device and given month/year from the database
	rows, err := db.Query(`
		SELECT id, device_id, user_id, start_date, end_date
		FROM bookings
		WHERE device_id = $1 AND EXTRACT(MONTH FROM start_date) = $2 AND EXTRACT(YEAR FROM start_date) = $3
	`, deviceID, int(month), year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var booking Booking
		err := rows.Scan(&booking.ID, &booking.DeviceID, &booking.UserID, &booking.StartDate, &booking.EndDate)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, booking)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return bookings, nil
}

func getUserID(r *http.Request) int {
	// Retrieve the user ID from the authenticated user's session
	cookie, err := r.Cookie("authenticated")
	if err != nil {
		return 0
	}

	// Parse the user ID from the cookie value
	userID, err := strconv.Atoi(cookie.Value)
	if err != nil {
		return 0
	}

	return userID
}

func renderTemplate(w http.ResponseWriter, templateFile string, data interface{}) {
	tmpl, err := template.ParseFiles(templateFile, "menu.html")
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
