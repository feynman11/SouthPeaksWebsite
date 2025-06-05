package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil" // For reading response body
	"log"
	"net/http"
	"strconv"
	"strings" // For string manipulation (search)
	"time"

	// "github.com/gorilla/sessions" // This import is not needed here as 'store' is global in main.go
	"golang.org/x/oauth2"
)

// Represents a Strava Athlete response (simplified)
type StravaAthlete struct {
	ID        int64  `json:"id"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Profile   string `json:"profile"` // URL to profile picture
}

// Represents a Strava Route from the API (simplified)
type StravaRouteAPI struct {
	ID            int64       `json:"id"`
	Name          string      `json:"name"`
	Distance      float64     `json:"distance"`       // Meters
	ElevationGain float64     `json:"elevation_gain"` // Meters
	Type          interface{} `json:"type"`           // Can be string or number from Strava API
	SubType       interface{} `json:"sub_type"`       // Can be string or number from Strava API
	// You might add more fields from Strava API if needed for display
	// e.g., Map struct for polyline, segments
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

	http.Redirect(w, r, "/", http.StatusFound) // Redirect to home page
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
		IsAdmin:     user.IsAdmin,
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
	if !isLoggedIn || !user.IsAdmin { // Must be logged in AND an admin
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

	// After submission, re-fetch all members to re-render the list dynamically via HTMX
	members, err := GetAllUsers(ctx)
	if err != nil {
		log.Printf("Error fetching all members after toggle: %v", err)
		http.Error(w, "Failed to load updated members list", http.StatusInternalServerError)
		return
	}

	data := TemplateData{ // Populate data for the fragment
		IsAdmin: user.IsAdmin,
		Members: members,
	}

	// Render only the members_grid_fragment.html template for HTMX swap
	w.Header().Set("Content-Type", "text/html")
	err = tmpl.ExecuteTemplate(w, "members_grid_fragment.html", data)
	if err != nil {
		log.Printf("Error executing members_grid_fragment template: %v", err)
		http.Error(w, "Failed to render updated list", http.StatusInternalServerError)
	}
}

// deleteAccountHandler handles user account deletion
func deleteAccountHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "session-name")
	if err != nil {
		log.Printf("Error getting session for delete: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

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
	session.Values["userID"] = nil
	session.Options.MaxAge = -1
	if err := session.Save(r, w); err != nil {
		log.Printf("Error saving session after delete: %v", err)
	}

	log.Printf("Account for Strava ID %d deleted successfully.", userID)

	// Use HX-Redirect for a clean browser-side redirect
	w.Header().Set("HX-Redirect", "/") // Tell HTMX to redirect the browser to the home page
	w.WriteHeader(http.StatusOK)
	return
}

// routesHandler displays the routes page
func routesHandler(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := getUserFromSession(r)
	if !isLoggedIn {
		http.Redirect(w, r, "/login/strava", http.StatusFound) // Must be logged in to view routes
		return
	}

	ctx := r.Context()
	routes, err := GetAllRoutes(ctx) // All club routes from Firestore
	if err != nil {
		log.Printf("Error fetching all club routes for routes page: %v", err)
		http.Error(w, "Failed to load club routes list", http.StatusInternalServerError)
		return
	}

	userSubmittedRoutes := []Route{} // Routes previously submitted by current user to the club
	if isLoggedIn { // Only fetch user's routes if logged in
		// Convert int64 StravaID to string for GetUserRoutes
		userSubmittedRoutes, err = GetUserRoutes(ctx, strconv.FormatInt(user.StravaID, 10))
		if err != nil { // Corrected: Using 'err' here
			log.Printf("Error fetching user's previously submitted routes: %v", err) // Corrected: Using 'err' here
		}
	}

	// This is for the initial load of the Strava routes dropdown.
	// It's still necessary here for the initial page render.
	stravaUserRoutesForDropdown := []StravaRouteAPI{}
	if isLoggedIn && user.IsPaidMember { // Only fetch Strava routes if paid member
		accessToken, err := GetFreshStravaToken(ctx, user)
		if err != nil {
			log.Printf("Error getting fresh Strava token for routes page initial load: %v", err)
			// User will see an error in the form, but page will still load other content
		} else {
			var fetchErr error
			stravaUserRoutesForDropdown, fetchErr = fetchStravaUserRoutes(ctx, accessToken, user.StravaID)
			if fetchErr != nil {
				log.Printf("Error fetching Strava routes for routes page initial load: %v", fetchErr)
				// User will see an error in the form if routes couldn't be loaded
			}
		}
	}

	data := TemplateData{
		Location:    "Borrowash, Derbyshire",
		CurrentYear: time.Now().Year(),
		IsLoggedIn:  true,
		User:        user,
		IsAdmin:     user.IsAdmin,
		Routes:      routes,                      // All club routes
		UserRoutes:  userSubmittedRoutes,         // User's previously submitted club routes
		StravaUserRoutes: stravaUserRoutesForDropdown, // For initial dropdown population
	}

	// Remove: w.WriteHeader(http.StatusOK) HERE, it's superfluous as tmpl.ExecuteTemplate does it
	err = tmpl.ExecuteTemplate(w, "routes.html", data) // Render routes template
	if err != nil {
		log.Printf("Error executing routes template: %v", err)
		// Only send http.Error if template execution fails
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// fetchStravaUserRoutes makes an API call to Strava to get a user's routes.
// This function fetches up to per_page=200 routes. For pagination, would need more logic.
func fetchStravaUserRoutes(ctx context.Context, accessToken string, athleteID int64) ([]StravaRouteAPI, error) {
	client := stravaOAuthConf.Client(ctx, &oauth2.Token{AccessToken: accessToken})

	url := fmt.Sprintf("https://www.strava.com/api/v3/athletes/%d/routes?per_page=200", athleteID)

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get athlete routes from Strava API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Strava API route fetch failed for user %d. Status: %d, Body: %s", athleteID, resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("strava API route fetch returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var stravaRoutes []StravaRouteAPI
	if err := json.NewDecoder(resp.Body).Decode(&stravaRoutes); err != nil {
		return nil, fmt.Errorf("failed to decode Strava routes JSON: %w", err)
	}

	return stravaRoutes, nil
}

// searchStravaRoutesHandler handles HTMX requests to search/filter Strava routes for a user
// It returns HTML <option> tags to update the select dropdown.
func searchStravaRoutesHandler(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := getUserFromSession(r)
	if !isLoggedIn {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !user.IsPaidMember {
		http.Error(w, "Forbidden: Only paid members can search Strava routes", http.StatusForbidden)
		return
	}

	query := r.URL.Query().Get("q") // The search query from HTMX (e.g., hx-trigger="keyup changed" or hx-trigger="search")
	ctx := r.Context()

	// This is the source of truth for the dropdown, fetched directly from Strava
	accessToken, err := GetFreshStravaToken(ctx, user)
	if err != nil {
		log.Printf("Error getting fresh Strava token for search: %v", err)
		w.Write([]byte(`<option value="">-- Failed to load routes --</option>`)) // HTMX expects options
		return
	}

	allStravaRoutes, fetchErr := fetchStravaUserRoutes(ctx, accessToken, user.StravaID) // Fetches all up to per_page limit
	if fetchErr != nil {
		log.Printf("Error fetching all Strava routes for search: %v", fetchErr)
		w.Write([]byte(`<option value="">-- Error fetching routes --</option>`)) // HTMX expects options
		return
	}

	filteredRoutes := []StravaRouteAPI{}
	if query == "" {
		// If no query, return ALL fetched Strava routes by default
		filteredRoutes = allStravaRoutes
	} else {
		lowerQuery := strings.ToLower(query)
		for _, route := range allStravaRoutes {
			if strings.Contains(strings.ToLower(route.Name), lowerQuery) {
				filteredRoutes = append(filteredRoutes, route)
			}
		}
	}

	var optionsHTML strings.Builder
	optionsHTML.WriteString(`<option value="">-- Select a Strava Route --</option>`) // Always include an empty default

	if len(filteredRoutes) == 0 {
		// This block covers both "no routes found for search" and "no routes at all"
		if query != "" {
			optionsHTML.WriteString(fmt.Sprintf(`<option value="" disabled>-- No matching routes for "%s" --</option>`, query))
		} else {
			optionsHTML.WriteString(`<option value="" disabled>-- No Strava Routes found --</option>`) // If no routes at all
		}
	} else {
		for _, route := range filteredRoutes {
			optionsHTML.WriteString(fmt.Sprintf(
				`<option value="%d">%s (%.1fkm, %.0fm Gain)</option>`,
				route.ID,
				route.Name,
				route.Distance/1000, // Convert meters to kilometers
				route.ElevationGain,
			))
		}
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(optionsHTML.String()))
}


// submitRouteHandler handles the submission (creation or re-classification) of routes
func submitRouteHandler(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := getUserFromSession(r)
	if !isLoggedIn {
		http.Error(w, "Unauthorized: Not logged in", http.StatusUnauthorized)
		return
	}
	if !user.IsPaidMember {
		http.Error(w, "Forbidden: Only paid members can submit or modify routes", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	selectedRouteID := r.FormValue("selectedRouteID")       // From "My Submitted Routes" dropdown
	stravaRouteSelectID := r.FormValue("stravaRouteSelect") // From Strava API dropdown
	routeClassify := r.FormValue("routeClassify")

	ctx := r.Context()
	var routeToSave *Route // Will hold the route to create or update

	if selectedRouteID != "" {
		// --- Scenario 1: User is re-classifying one of their existing submitted club routes ---
		existingRoute, err := GetRouteByID(ctx, selectedRouteID)
		if err != nil {
			log.Printf("Error getting existing route %s for re-classification: %v", selectedRouteID, err)
			http.Error(w, "Failed to retrieve existing route", http.StatusInternalServerError)
			return
		}

		// Authorization check: ensure user owns this route, or is an admin
		if existingRoute.SubmittedByUserID != strconv.FormatInt(user.StravaID, 10) && !user.IsAdmin {
			http.Error(w, "Forbidden: You can only re-classify your own routes", http.StatusForbidden)
			return
		}

		existingRoute.Classify = routeClassify // Update classification
		routeToSave = existingRoute // Use existing route for update
	} else if stravaRouteSelectID != "" {
		// --- Scenario 2: User is adding a route from their Strava list ---
		stravaRouteID, err := strconv.ParseInt(stravaRouteSelectID, 10, 64)
		if err != nil {
			http.Error(w, "Invalid Strava route ID selected", http.StatusBadRequest)
			return
		}

		// Fetch the selected Strava route details using the fresh token
		accessToken, tokenErr := GetFreshStravaToken(ctx, user)
		if tokenErr != nil {
			log.Printf("Error getting fresh Strava token for route fetch: %v", tokenErr)
			http.Error(w, "Failed to authenticate with Strava API", http.StatusInternalServerError)
			return
		}
		
		specificRouteURL := fmt.Sprintf("https://www.strava.com/api/v3/routes/%d", stravaRouteID)

		client := stravaOAuthConf.Client(ctx, &oauth2.Token{AccessToken: accessToken})
		resp, err := client.Get(specificRouteURL)
		if err != nil {
			log.Printf("Error fetching specific Strava route %d details: %v", stravaRouteID, err)
			http.Error(w, "Failed to get route details from Strava API", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			log.Printf("Strava API specific route fetch failed. Status: %d, Body: %s", resp.StatusCode, string(bodyBytes))
			http.Error(w, "Failed to retrieve Strava route details", http.StatusInternalServerError)
			return
		}

		var stravaRouteDetail StravaRouteAPI
		if err := json.NewDecoder(resp.Body).Decode(&stravaRouteDetail); err != nil {
			log.Printf("Failed to decode specific Strava route details: %v", err)
			http.Error(w, "Failed to parse Strava route data", http.StatusInternalServerError)
			return
		}

		routeToSave = &Route{
			Name:                stravaRouteDetail.Name,
			URL:                 fmt.Sprintf("https://www.strava.com/routes/%d", stravaRouteDetail.ID),
			SubmittedByUserID:   strconv.FormatInt(user.StravaID, 10),
			SubmittedByUserName: fmt.Sprintf("%s %s", user.FirstName, user.LastName),
			SubmittedAt:         time.Now(),
		}
	} else {
		// If neither selectedRouteID nor stravaRouteSelectID is present, it's an invalid submission.
		http.Error(w, "No route selected or invalid submission method", http.StatusBadRequest)
		return
	}

	// Apply classification (from dropdown for both new and existing submissions)
	if routeClassify != "Thursday" && routeClassify != "Saturday" && routeClassify != "Other" {
		http.Error(w, "Invalid route classification", http.StatusBadRequest)
		return
	}
	routeToSave.Classify = routeClassify

	// Save or update the route in Firestore
	if err := CreateRoute(ctx, routeToSave); err != nil {
		log.Printf("Error creating/updating route in Firestore: %v", err)
		http.Error(w, "Failed to submit/update route", http.StatusInternalServerError)
		return
	}

	log.Printf("Route submitted/updated by %s: %s (Classify: %s, ID: %s)", routeToSave.SubmittedByUserName, routeToSave.Name, routeToSave.Classify, routeToSave.ID)

	// After submission/deletion, re-fetch all routes once and filter for user's routes for HTMX response
	allRoutes, err := GetAllRoutes(ctx)
	if err != nil {
		log.Printf("Error fetching all routes after submission: %v", err)
		http.Error(w, "Failed to load updated routes list", http.StatusInternalServerError)
		return
	}

	filteredUserRoutes := []Route{}
	loggedInUserIDStr := strconv.FormatInt(user.StravaID, 10)
	for _, r := range allRoutes {
		if r.SubmittedByUserID == loggedInUserIDStr {
			filteredUserRoutes = append(filteredUserRoutes, r)
		}
	}

	data := TemplateData{ // Populate data for the fragment
		IsLoggedIn: true,
		User:       user,
		IsAdmin:    user.IsAdmin,
		Routes:     allRoutes,
		UserRoutes: filteredUserRoutes,
		// StravaUserRoutes is only needed for initial routes page load and dynamic search
	}

	w.Header().Set("Content-Type", "text/html")
	err = tmpl.ExecuteTemplate(w, "routes_list_fragment.html", data)
	if err != nil {
		log.Printf("Error executing routes_list_fragment template: %v", err)
		http.Error(w, "Failed to render updated routes list", http.StatusInternalServerError)
	}
}

// deleteRouteHandler handles deletion of a route
func deleteRouteHandler(w http.ResponseWriter, r *http.Request) {
	user, isLoggedIn := getUserFromSession(r)
	if !isLoggedIn {
		http.Error(w, "Unauthorized: Not logged in", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	routeID := r.FormValue("routeID")
	if routeID == "" {
		http.Error(w, "Route ID missing", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	routeToDelete, err := GetRouteByID(ctx, routeID)
	if err != nil {
		log.Printf("Error getting route %s for deletion: %v", routeID, err)
		http.Error(w, "Route not found", http.StatusNotFound)
		return
	}

	// Authorization check: User can only delete their own route unless they are admin
	if routeToDelete.SubmittedByUserID != strconv.FormatInt(user.StravaID, 10) && !user.IsAdmin {
		http.Error(w, "Forbidden: You can only delete your own routes.", http.StatusForbidden)
		return
	}

	if err := DeleteRoute(ctx, routeID); err != nil {
		log.Printf("Error deleting route %s from Firestore: %v", routeID, err)
		http.Error(w, "Failed to delete route from database", http.StatusInternalServerError)
		return
	}

	log.Printf("Route %s deleted by user %s (Admin: %t).", routeID, user.FirstName, user.IsAdmin)

	// After deletion, re-fetch all routes once and filter for user's routes for HTMX response
	allRoutes, err := GetAllRoutes(ctx)
	if err != nil {
		log.Printf("Error fetching all routes after deletion: %v", err)
		http.Error(w, "Failed to load updated routes list", http.StatusInternalServerError)
		return
	}

	filteredUserRoutes := []Route{}
	loggedInUserIDStr := strconv.FormatInt(user.StravaID, 10)
	for _, r := range allRoutes {
		if r.SubmittedByUserID == loggedInUserIDStr {
			filteredUserRoutes = append(filteredUserRoutes, r)
		}
	}

	data := TemplateData{ // Populate data for the fragment
		IsLoggedIn: true,
		User:       user,
		IsAdmin:    user.IsAdmin,
		Routes:     allRoutes,
		UserRoutes: filteredUserRoutes,
		// StravaUserRoutes is only needed for initial routes page load and dynamic search
	}

	w.Header().Set("Content-Type", "text/html")
	err = tmpl.ExecuteTemplate(w, "routes_list_fragment.html", data)
	if err != nil {
		log.Printf("Error executing routes_list_fragment template: %v", err)
		http.Error(w, "Failed to render updated routes list", http.StatusInternalServerError)
	}
}