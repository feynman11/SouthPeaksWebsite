package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"
)

// Represents a Strava Athlete response (simplified)
type StravaAthlete struct {
	ID        int64  `json:"id"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Profile   string `json:"profile"` // URL to profile picture
}

// stravaLoginHandler redirects user to Strava for OAuth authorization
func stravaLoginHandler(w http.ResponseWriter, r *http.Request) {
	// Generate a random state string to prevent CSRF attacks
	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)

	// Save state to session
	session, _ := store.Get(r, "session-name")
	session.Values["oauthState"] = state
	session.Save(r, w)

	url := stravaOAuthConf.AuthCodeURL(state, oauth2.AccessTypeOffline) // Request refresh token
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// stravaCallbackHandler handles the redirect from Strava after authorization
func stravaCallbackHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")

	// Verify state to prevent CSRF
	if r.FormValue("state") != session.Values["oauthState"] {
		http.Error(w, "Invalid state", http.StatusUnauthorized)
		return
	}

	// Exchange authorization code for tokens
	code := r.FormValue("code")
	if code == "" {
		http.Error(w, "Authorization code not provided", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	token, err := stravaOAuthConf.Exchange(ctx, code)
	if err != nil {
		log.Printf("Error exchanging code for token: %v", err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	// Use the access token to get athlete details
	athlete, err := getStravaAthleteDetails(ctx, token.AccessToken)
	if err != nil {
		log.Printf("Error fetching Strava athlete details: %v", err)
		http.Error(w, "Failed to get athlete details", http.StatusInternalServerError)
		return
	}

	// Check if user exists in Firestore, create or update
	user, err := GetUserByID(ctx, athlete.ID)
	if err != nil && err.Error() == "user not found" {
		// User does not exist, create new
		user = &User{
			StravaID:       athlete.ID,
			FirstName:      athlete.FirstName,
			LastName:       athlete.LastName,
			ProfilePicURL:  athlete.Profile,
			IsPaidMember:   false, // Default to not paid
			IsAdmin:        false, // Default to not admin
			LastLogin:      time.Now(),
			AccessToken:    token.AccessToken,
			RefreshToken:   token.RefreshToken,
			AccessTokenExp: token.Expiry,
		}
		if err := CreateUser(ctx, user); err != nil {
			log.Printf("Error creating user in Firestore: %v", err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}
		log.Printf("New user registered: %s %s (Strava ID: %d)", user.FirstName, user.LastName, user.StravaID)
	} else if err != nil {
		log.Printf("Error getting user from Firestore: %v", err)
		http.Error(w, "Failed to retrieve user data", http.StatusInternalServerError)
		return
	} else {
		// User exists, update tokens and last login time
		user.AccessToken = token.AccessToken
		user.RefreshToken = token.RefreshToken
		user.AccessTokenExp = token.Expiry
		user.LastLogin = time.Now()
		if err := UpdateUser(ctx, user); err != nil {
			log.Printf("Error updating user in Firestore: %v", err)
			http.Error(w, "Failed to update user data", http.StatusInternalServerError)
			return
		}
		log.Printf("User logged in: %s %s (Strava ID: %d)", user.FirstName, user.LastName, user.StravaID)
	}

	// Set user ID in session
	session.Values["userID"] = athlete.ID
	session.Save(r, w)

	http.Redirect(w, r, "/", http.StatusFound) 
}

// getStravaAthleteDetails fetches the current athlete's details using their access token
func getStravaAthleteDetails(ctx context.Context, accessToken string) (*StravaAthlete, error) {
	client := stravaOAuthConf.Client(ctx, &oauth2.Token{AccessToken: accessToken})
	resp, err := client.Get("https://www.strava.com/api/v3/athlete")
	if err != nil {
		return nil, fmt.Errorf("failed to get athlete details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("strava API returned status %d", resp.StatusCode)
	}

	var athlete StravaAthlete
	if err := json.NewDecoder(resp.Body).Decode(&athlete); err != nil {
		return nil, fmt.Errorf("failed to decode athlete details: %w", err)
	}
	return &athlete, nil
}

// logoutHandler clears the user session
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	session.Values["userID"] = nil // Clear user ID
	session.Options.MaxAge = -1    // Immediately expire the cookie
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

// membersHandler displays the members page (requires login)
func membersHandler(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := getUserFromSession(r) // Get user from session
	if !isLoggedIn {
		http.Redirect(w, r, "/login/strava", http.StatusFound) // Redirect to login if not authenticated
		return
	}

	ctx := r.Context()
	members, err := GetAllUsers(ctx) // Fetch all users from Firestore
	if err != nil {
		log.Printf("Error fetching all members: %v", err)
		http.Error(w, "Failed to load members list", http.StatusInternalServerError)
		return
	}

	data := TemplateData{
		Location:    "Borrowash, Derbyshire",
		CurrentYear: time.Now().Year(),
		IsLoggedIn:  true,
		User:        user,
		IsAdmin:     user.IsAdmin, // Pass admin status to template
		Members:     members,
	}

	err = tmpl.ExecuteTemplate(w, "members.html", data) // Render members template
	if err != nil {
		log.Printf("Error executing members template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}


// adminTogglePaidHandler allows an admin to toggle paid status for a member
func adminTogglePaidHandler(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := getUserFromSession(r)
	if !isLoggedIn || !user.IsAdmin {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	targetUserIDStr := r.FormValue("userID")
	targetUserID, err := strconv.ParseInt(targetUserIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	targetUser, err := GetUserByID(ctx, targetUserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Toggle paid status
	targetUser.IsPaidMember = !targetUser.IsPaidMember
	if err := UpdateUser(ctx, targetUser); err != nil {
		log.Printf("Error toggling paid status for user %d: %v", targetUserID, err)
		http.Error(w, "Failed to update paid status", http.StatusInternalServerError)
		return
	}

	// --- NEW: Fetch all members again to re-render the list ---
	members, err := GetAllUsers(ctx) // Re-fetch data after update
	if err != nil {
		log.Printf("Error fetching all members after toggle: %v", err)
		http.Error(w, "Failed to re-load members list", http.StatusInternalServerError)
		return
	}

	data := TemplateData{ // Populate data for the fragment
		IsAdmin: user.IsAdmin, // Ensure admin status is passed to the fragment
		Members: members,
	}

	// --- NEW: Render only the members_grid_fragment.html template ---
	// Set Content-Type header to tell HTMX it's HTML
	w.Header().Set("Content-Type", "text/html")
	err = tmpl.ExecuteTemplate(w, "members_grid_fragment.html", data) // Render the fragment
	if err != nil {
		log.Printf("Error executing members_grid_fragment template: %v", err)
		http.Error(w, "Failed to render updated list", http.StatusInternalServerError)
	}
	// No explicit return needed if ExecuteTemplate completes successfully, as it's the last action.
}

// deleteAccountHandler allows users to delete their account
func deleteAccountHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "session-name")
	if err != nil {
		log.Printf("Error getting session for delete: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	// Must be logged in
	userIDVal := session.Values["userID"]
	if userIDVal == nil {
		http.Error(w, "Not logged in", http.StatusUnauthorized)
		return
	}
	userID := userIDVal.(int64)

	ctx := r.Context()

	// 1. Delete user from Firestore
	if err := DeleteUser(ctx, userID); err != nil {
		log.Printf("Error deleting user %d from Firestore: %v", userID, err)
		http.Error(w, "Failed to delete account from database", http.StatusInternalServerError)
		return
	}

	// 2. Clear user session
	session.Values["userID"] = nil // Clear user ID
	session.Options.MaxAge = -1    // Immediately expire the cookie
	if err := session.Save(r, w); err != nil {
		log.Printf("Error saving session after delete: %v", err)
		// Try to continue redirecting even if session save fails, as account is deleted
	}

	log.Printf("Account for Strava ID %d deleted successfully.", userID)

	w.Header().Set("HX-Redirect", "/") // Tell HTMX to redirect the browser to the home page
	w.WriteHeader(http.StatusOK)       // Send a 200 OK or 204 No Content
	return
}