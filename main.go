package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

// --- Configuration Constants (Move to environment variables in production!) ---
var (
	stravaClientID     = os.Getenv("STRAVA_CLIENT_ID")
	stravaClientSecret = os.Getenv("STRAVA_CLIENT_SECRET")
	sessionSecretKey   = os.Getenv("SESSION_SECRET_KEY") // A long, random string
	// IMPORTANT: Set this to your app's actual URL for OAuth callback!
	// For local: http://localhost:8080
	// For Cloud Run/App Engine: https://your-custom-domain.com or https://your-app-id.run.app
	oauthCallbackURL = os.Getenv("OAUTH_CALLBACK_URL")

	// Store for sessions (e.g., cookie store)
	// For production, consider using a secure, distributed store like Firestore or Memorystore
	store = sessions.NewCookieStore([]byte(sessionSecretKey))

	// OAuth2 configuration for Strava
	stravaOAuthConf *oauth2.Config

	// Firestore client
	firestoreClient *firestore.Client
)

// TemplateData holds data to be passed to HTML templates
type TemplateData struct {
	Location     string
	Tagline      string
	StravaURL    string
	InstagramURL string
	CurrentYear  int
	// Add user-specific data for templates
	IsLoggedIn bool
	User       *User // User info from session
	IsAdmin    bool
	Members    []User // For the members page
}

var tmpl *template.Template

func main() {
	// Initialize configuration
	if stravaClientID == "" || stravaClientSecret == "" || sessionSecretKey == "" || oauthCallbackURL == "" {
		log.Fatal("Missing environment variables: STRAVA_CLIENT_ID, STRAVA_CLIENT_SECRET, SESSION_SECRET_KEY, OAUTH_CALLBACK_URL")
	}

	stravaOAuthConf = &oauth2.Config{
		ClientID:     stravaClientID,
		ClientSecret: stravaClientSecret,
		RedirectURL:  oauthCallbackURL + "/auth/strava/callback",
		Scopes:       []string{"read_all"}, // Request read_all access to Strava data
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.strava.com/oauth/authorize",
			TokenURL: "https://www.strava.com/oauth/token",
		},
	}

	// Initialize Firestore
	ctx := context.Background()
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT") // App Engine/Cloud Run sets this
	if projectID == "" {
		// Fallback for local development if GOOGLE_CLOUD_PROJECT not set
		log.Println("GOOGLE_CLOUD_PROJECT not set, attempting default Firestore client")
		firestoreClient, _ = firestore.NewClient(ctx, "southpeakswebsite") // Replace with your actual project ID for local testing
	} else {
		var err error
		firestoreClient, err = firestore.NewClient(ctx, projectID)
		if err != nil {
			log.Fatalf("Failed to create Firestore client: %v", err)
		}
	}
	defer firestoreClient.Close()

	// Parse templates
	tmpl = template.Must(template.ParseGlob(filepath.Join("templates", "*.html"))) // Parse all HTML files in templates

	// Serve static files (CSS, Images, etc.)
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// --- Routes ---
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/login/strava", stravaLoginHandler)
	http.HandleFunc("/auth/strava/callback", stravaCallbackHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/members", membersHandler) // New members page
	http.HandleFunc("/admin/toggle-paid", adminTogglePaidHandler) // Admin action

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

// Global variable for current user data (fetched from session middleware)
// This is not ideal for concurrent access in real app, but for simple template use...
// A better pattern would be to pass user data via request context.
var currentAuthUser *User
var currentIsAdmin bool

// getUserFromSession is a middleware-like function to populate currentAuthUser
func getUserFromSession(r *http.Request) (*User, bool) {
	session, err := store.Get(r, "session-name") // "session-name" is arbitrary
	if err != nil {
		log.Printf("Error getting session: %v", err)
		return nil, false
	}

	userIDVal := session.Values["userID"]
	if userIDVal == nil {
		return nil, false
	}
	userID := userIDVal.(int64) // Strava Athlete ID is int64

	ctx := r.Context()
	user, err := GetUserByID(ctx, userID)
	if err != nil {
		log.Printf("Error getting user from Firestore by ID %d: %v", userID, err)
		return nil, false
	}
	currentAuthUser = user // Set global for template access (see note above)
	currentIsAdmin = user.IsAdmin
	return user, true
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	user, isLoggedIn := getUserFromSession(r) // Try to get user from session

	data := TemplateData{
		Location:     "Borrowash, Derbyshire",
		Tagline:      "Ride Together, Grow Together",
		StravaURL:    "https://www.strava.com/clubs/451869",
		InstagramURL: "https://www.instagram.com/southpeakscc",
		CurrentYear:  time.Now().Year(),
		IsLoggedIn:   isLoggedIn,
		User:         user,
		IsAdmin:      currentIsAdmin, // Assuming currentIsAdmin is set by getUserFromSession
	}

	err := tmpl.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}