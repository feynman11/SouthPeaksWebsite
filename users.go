package main

import (
	"context"
	"errors"
	"fmt"
	"time"
	"log"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"golang.org/x/oauth2" 
)

// User represents a user stored in Firestore
type User struct {
	StravaID       int64     `firestore:"stravaID"`
	FirstName      string    `firestore:"firstName"`
	LastName       string    `firestore:"lastName"`
	ProfilePicURL  string    `firestore:"profilePicURL"`
	IsPaidMember   bool      `firestore:"isPaidMember"`
	IsAdmin        bool      `firestore:"isAdmin"`
	LastLogin      time.Time `firestore:"lastLogin"`
	AccessToken    string    `firestore:"accessToken"`    // Stored token
	RefreshToken   string    `firestore:"refreshToken"`   // Stored token
	AccessTokenExp time.Time `firestore:"accessTokenExp"` // When token expires
}

const usersCollection = "users" // Firestore collection name

// GetUserByID retrieves a user by their StravaID from Firestore
func GetUserByID(ctx context.Context, stravaID int64) (*User, error) {
	docRef := firestoreClient.Collection(usersCollection).Doc(fmt.Sprintf("%d", stravaID))
	docSnap, err := docRef.Get(ctx)
	if err != nil {
		if errors.Is(err, iterator.Done) || docSnap == nil || !docSnap.Exists() {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user document: %w", err)
	}

	var user User
	if err := docSnap.DataTo(&user); err != nil {
		return nil, fmt.Errorf("failed to convert document to user: %w", err)
	}
	return &user, nil
}

// CreateUser creates a new user document in Firestore
func CreateUser(ctx context.Context, user *User) error {
	docRef := firestoreClient.Collection(usersCollection).Doc(fmt.Sprintf("%d", user.StravaID))
	_, err := docRef.Set(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to create user document: %w", err)
	}
	return nil
}

// UpdateUser updates an existing user document in Firestore
func UpdateUser(ctx context.Context, user *User) error {
	docRef := firestoreClient.Collection(usersCollection).Doc(fmt.Sprintf("%d", user.StravaID))
	_, err := docRef.Set(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to update user document: %w", err)
	}
	return nil
}

// GetAllUsers retrieves all users from Firestore
func GetAllUsers(ctx context.Context) ([]User, error) {
	var users []User
	iter := firestoreClient.Collection(usersCollection).OrderBy("firstName", firestore.Asc).Documents(ctx)
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error iterating over users: %w", err)
		}

		var user User
		if err := doc.DataTo(&user); err != nil {
			return nil, fmt.Errorf("error converting document to user: %w", err)
		}
		users = append(users, user)
	}
	return users, nil
}

// DeleteUser deletes a user document from Firestore
func DeleteUser(ctx context.Context, stravaID int64) error {
	docRef := firestoreClient.Collection(usersCollection).Doc(fmt.Sprintf("%d", stravaID))
	_, err := docRef.Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete user document with ID %d: %w", stravaID, err)
	}
	return nil
}
// RefreshStravaToken attempts to refresh an expired Strava access token
// It updates the user's document in Firestore with the new tokens.
func RefreshStravaToken(ctx context.Context, user *User) error {
	// Use oauth2.Config to get a token source
	tokenSource := stravaOAuthConf.TokenSource(ctx, &oauth2.Token{
		AccessToken:  user.AccessToken,
		RefreshToken: user.RefreshToken,
		Expiry:       user.AccessTokenExp,
		TokenType:    "Bearer", // Strava uses Bearer tokens
	})

	// Request a fresh token. If the old one is expired, it will use the refresh token.
	newToken, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to refresh Strava token for user %d: %w", user.StravaID, err)
	}

	// If a new token was obtained, update the user in Firestore
	if newToken.AccessToken != user.AccessToken || newToken.RefreshToken != user.RefreshToken || !newToken.Expiry.Equal(user.AccessTokenExp) {
		user.AccessToken = newToken.AccessToken
		user.RefreshToken = newToken.RefreshToken
		user.AccessTokenExp = newToken.Expiry
		if err := UpdateUser(ctx, user); err != nil {
			return fmt.Errorf("failed to update user tokens in Firestore after refresh: %w", err)
		}
		log.Printf("Successfully refreshed Strava token for user %d. New expiry: %s", user.StravaID, newToken.Expiry.Format(time.RFC3339))
	} else {
		log.Printf("Strava token for user %d still valid or no refresh needed.", user.StravaID)
	}
	return nil
}

// Ensure token is fresh before making API calls.
// This is a helper that tries to refresh the token if it's near expiry.
func GetFreshStravaToken(ctx context.Context, user *User) (string, error) {
	// Give a buffer for expiry (e.g., 5 minutes before actual expiry)
	if time.Now().Add(5*time.Minute).After(user.AccessTokenExp) {
		log.Printf("Strava token for user %d is near expiry. Attempting refresh...", user.StravaID)
		if err := RefreshStravaToken(ctx, user); err != nil {
			return "", fmt.Errorf("failed to refresh Strava token: %w", err)
		}
	}
	return user.AccessToken, nil
}