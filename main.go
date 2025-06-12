package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gorilla/sessions"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/oauth2"
)

// --- Configuration Constants ---
var (
	stravaClientID     = os.Getenv("STRAVA_CLIENT_ID")
	stravaClientSecret = os.Getenv("STRAVA_CLIENT_SECRET")
	sessionSecretKey   = os.Getenv("SESSION_SECRET_KEY")
	oauthCallbackURL   = os.Getenv("OAUTH_CALLBACK_URL") // e.g., "https://www.southpeakscc.co.uk" or "http://localhost:8081"

	store           = sessions.NewCookieStore([]byte(sessionSecretKey))
	stravaOAuthConf *oauth2.Config
	mongoClient     *mongo.Client
	mongoDB         *mongo.Database
	cssVersion      = fmt.Sprintf("%d", time.Now().Unix()) // Use Unix timestamp as string for cache busting
)

// TemplateData holds data to be passed to HTML templates
type TemplateData struct {
	Location         string
	StravaURL        string
	InstagramURL     string
	CurrentYear      int
	IsLoggedIn       bool
	User             *User
	IsAdmin          bool
	Members          []User  // For members page (all members)
	Routes           []Route // For routes page (all club routes)
	UserRoutes       []Route // For routes page (user's own submitted routes)
	StravaUserRoutes string
	CSSVersion       string // Add this line
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
		Scopes:       []string{"read_all"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.strava.com/oauth/authorize",
			TokenURL: "https://www.strava.com/oauth/token",
		},
	}

	ctx := context.Background()
	var err error

	// Initialize MongoDB
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		log.Fatal("Missing MONGODB_URI environment variable")
	}
	mongoClient, err = mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	mongoDB = mongoClient.Database("southpeakscc") // or your db name

	defer func() {
		log.Println("Closing MongoDB client...")
		if err := mongoClient.Disconnect(ctx); err != nil {
			log.Printf("Error closing MongoDB client: %v", err)
		}
	}()

	// Parse templates - will parse all HTML files in templates directory
	tmpl = template.Must(template.ParseGlob(filepath.Join("templates", "*.html")))

	mux := http.NewServeMux() // Use a new ServeMux for better control
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// --- Routes ---
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/login/strava", stravaLoginHandler)
	mux.HandleFunc("/auth/strava/callback", stravaCallbackHandler)
	mux.HandleFunc("/logout", logoutHandler)
	mux.HandleFunc("/members", membersHandler)
	mux.HandleFunc("/admin/toggle-paid", adminTogglePaidHandler)
	mux.HandleFunc("/members/delete-account", deleteAccountHandler)
	mux.HandleFunc("/routes", routesHandler)
	mux.HandleFunc("/routes/submit", submitRouteHandler)
	mux.HandleFunc("/routes/delete", deleteRouteHandler)
	mux.HandleFunc("/routes/search-strava", searchStravaRoutesHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
		log.Printf("Defaulting to port %s", port)
	}

	// --- Graceful Shutdown Setup ---
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux, // Use our custom ServeMux
	}

	// Create a channel to listen for OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Goroutine to start the server
	go func() {
		log.Printf("Listening on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on %s: %v", port, err)
		}
	}()

	// Block until we receive a signal
	<-stop

	// Create a deadline for the shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Println("Shutting down server...")
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	log.Println("Server exited gracefully")
}

// Global variable for current user data (fetched from session middleware)
// This is not ideal for concurrent access in real app, but for simple template use...
// A better pattern would be to pass user data via request context.
var currentAuthUser *User
var currentIsAdmin bool

// getUserFromSession is a middleware-like function to populate currentAuthUser
func getUserFromSession(r *http.Request) (*User, bool) {
	session, err := store.Get(r, "session-name")
	if err != nil {
		log.Printf("Error getting session: %v", err)
		return nil, false
	}

	userIDVal := session.Values["userID"]
	if userIDVal == nil {
		return nil, false
	}
	userID := userIDVal.(int64)

	ctx := r.Context()
	user, err := GetUserByID(ctx, userID)
	if err != nil {
		log.Printf("Error getting user from DB by ID %d: %v", userID, err)
		return nil, false
	}
	currentAuthUser = user
	currentIsAdmin = user.IsAdmin
	return user, true
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	user, isLoggedIn := getUserFromSession(r)

	data := TemplateData{
		Location:     "Borrowash, Derbyshire",
		StravaURL:    "https://www.strava.com/clubs/451869",
		InstagramURL: "https://www.instagram.com/southpeakscc",
		CurrentYear:  time.Now().Year(),
		IsLoggedIn:   isLoggedIn,
		User:         user,
		IsAdmin:      currentIsAdmin,
		CSSVersion:   cssVersion, // Use Unix timestamp for cache busting
	}

	err := tmpl.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
