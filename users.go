package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

// User represents a user stored in Firestore
type User struct {
	StravaID       int64     `firestore:"stravaID"`
	FirstName      string    `firestore:"firstName"`
	LastName       string    `firestore:"lastName"`
	ProfilePicURL  string    `firestore:"profilePicURL"`
	IsPaidMember   bool      `firestore:"isPaidMember"`
	IsAdmin        bool      `firestore:"isAdmin"`        // New field for admin status
	LastLogin      time.Time `firestore:"lastLogin"`
	AccessToken    string    `firestore:"accessToken"`    // Store for refreshing tokens
	RefreshToken   string    `firestore:"refreshToken"`   // Store for refreshing tokens
	AccessTokenExp time.Time `firestore:"accessTokenExp"` // When access token expires
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
	_, err := docRef.Set(ctx, user) // Set with merge=false replaces, but Set with struct usually updates all fields
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