package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// TemplateData holds data to be passed to HTML templates
type TemplateData struct {
	ClubName     string
	SPCCAbbrev   string
	CycleClub    string
	Location     string
	Tagline      string
	StravaURL    string
	InstagramURL string
	CurrentYear  int
}

var tmpl *template.Template

func main() {
	// Parse templates
	// Using Must so it panics if templates are not parsed correctly at startup
	tmpl = template.Must(template.ParseGlob(filepath.Join("templates", "*.html")))

	// Serve static files (CSS)
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Handlers
	http.HandleFunc("/", indexHandler)
	// Add more HTMX specific handlers here later if needed
	// e.g., http.HandleFunc("/load-more-rides", loadMoreRidesHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	data := TemplateData{
		ClubName:     "SOUTH PEAKS",
		SPCCAbbrev:   "SPCC",
		CycleClub:    "CYCLE CLUB",
		Location:     "Borrowash, Derbyshire",
		Tagline:      "Ride Together, Grow Together",
		StravaURL:    "https://www.strava.com/clubs/451869",
		InstagramURL: "https://www.instagram.com/southpeakscc",
		CurrentYear:  time.Now().Year(),
	}

	err := tmpl.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Example of a future HTMX handler (not used in the initial page load)
/*
func loadMoreRidesHandler(w http.ResponseWriter, r *http.Request) {
	// In a real app, you'd fetch data here
	// For now, just sending a simple HTML snippet
	htmlSnippet := `
		<div class="info-card">
			<h4>New Ride Added!</h4>
			<p>Exciting new route announced for next weekend. Check Strava for details!</p>
		</div>
	`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(htmlSnippet))
}
*/
