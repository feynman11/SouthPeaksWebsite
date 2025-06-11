package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/oauth2"
)

// User represents a user stored in MongoDB
type User struct {
	StravaID       int64     `bson:"stravaID"`
	FirstName      string    `bson:"firstName"`
	LastName       string    `bson:"lastName"`
	ProfilePicURL  string    `bson:"profilePicURL"`
	IsPaidMember   bool      `bson:"isPaidMember"`
	IsAdmin        bool      `bson:"isAdmin"`
	LastLogin      time.Time `bson:"lastLogin"`
	AccessToken    string    `bson:"accessToken"`    // Stored token
	RefreshToken   string    `bson:"refreshToken"`   // Stored token
	AccessTokenExp time.Time `bson:"accessTokenExp"` // When token expires
}

const usersCollection = "users" // MongoDB collection name

// GetUserByID retrieves a user by their StravaID from MongoDB
func GetUserByID(ctx context.Context, stravaID int64) (*User, error) {
	var user User
	filter := bson.M{"stravaID": stravaID}
	err := mongoDB.Collection(usersCollection).FindOne(ctx, filter).Decode(&user)
	if err == mongo.ErrNoDocuments {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user document: %w", err)
	}
	return &user, nil
}

// CreateUser creates a new user document in MongoDB
func CreateUser(ctx context.Context, user *User) error {
	_, err := mongoDB.Collection(usersCollection).InsertOne(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to create user document: %w", err)
	}
	return nil
}

// UpdateUser updates an existing user document in MongoDB
func UpdateUser(ctx context.Context, user *User) error {
	filter := bson.M{"stravaID": user.StravaID}
	update := bson.M{"$set": user}
	_, err := mongoDB.Collection(usersCollection).UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update user document: %w", err)
	}
	return nil
}

// GetAllUsers retrieves all users from MongoDB, ordered by firstName
func GetAllUsers(ctx context.Context) ([]User, error) {
	var users []User
	opts := options.Find().SetSort(bson.D{{Key: "firstName", Value: 1}})
	cursor, err := mongoDB.Collection(usersCollection).Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("error finding users: %w", err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var user User
		if err := cursor.Decode(&user); err != nil {
			return nil, fmt.Errorf("error decoding user: %w", err)
		}
		users = append(users, user)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}
	return users, nil
}

// DeleteUser deletes a user document from MongoDB
func DeleteUser(ctx context.Context, stravaID int64) error {
	filter := bson.M{"stravaID": stravaID}
	_, err := mongoDB.Collection(usersCollection).DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete user document with ID %d: %w", stravaID, err)
	}
	return nil
}

// RefreshStravaToken attempts to refresh an expired Strava access token
// It updates the user's document in MongoDB with the new tokens.
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

	// If a new token was obtained, update the user in MongoDB
	if newToken.AccessToken != user.AccessToken || newToken.RefreshToken != user.RefreshToken || !newToken.Expiry.Equal(user.AccessTokenExp) {
		user.AccessToken = newToken.AccessToken
		user.RefreshToken = newToken.RefreshToken
		user.AccessTokenExp = newToken.Expiry
		if err := UpdateUser(ctx, user); err != nil {
			return fmt.Errorf("failed to update user tokens in MongoDB after refresh: %w", err)
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
	if time.Now().Add(5 * time.Minute).After(user.AccessTokenExp) {
		log.Printf("Strava token for user %d is near expiry. Attempting refresh...", user.StravaID)
		if err := RefreshStravaToken(ctx, user); err != nil {
			return "", fmt.Errorf("failed to refresh Strava token: %w", err)
		}
	}
	log.Printf("Using Strava token for user %d", user.StravaID)

	return user.AccessToken, nil
}
