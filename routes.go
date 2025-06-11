package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Route represents a Strava route submitted by a member
type Route struct {
	ID                  string    `bson:"_id,omitempty"` // MongoDB document ID (as hex string)
	Name                string    `bson:"name"`
	URL                 string    `bson:"url"`
	Classify            string    `bson:"classify"`
	SubmittedByUserID   string    `bson:"submittedByUserID"`
	SubmittedByUserName string    `bson:"submittedByUserName"`
	SubmittedAt         time.Time `bson:"submittedAt"`
}

const routesCollection = "routes" // MongoDB collection name

// CreateRoute adds a new route document to MongoDB or updates an existing one
func CreateRoute(ctx context.Context, route *Route) error {
	coll := mongoDB.Collection(routesCollection)
	if route.ID == "" {
		route.SubmittedAt = time.Now()
		res, err := coll.InsertOne(ctx, route)
		if err != nil {
			return fmt.Errorf("failed to create route document: %w", err)
		}
		if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
			route.ID = oid.Hex()
		}
	} else {
		objID, err := primitive.ObjectIDFromHex(route.ID)
		if err != nil {
			return fmt.Errorf("invalid route ID: %w", err)
		}
		route.SubmittedAt = time.Now()
		_, err = coll.ReplaceOne(ctx, bson.M{"_id": objID}, route)
		if err != nil {
			return fmt.Errorf("failed to update route document %s: %w", route.ID, err)
		}
	}
	return nil
}

// GetRouteByID retrieves a single route by its MongoDB document ID
func GetRouteByID(ctx context.Context, routeID string) (*Route, error) {
	coll := mongoDB.Collection(routesCollection)
	objID, err := primitive.ObjectIDFromHex(routeID)
	if err != nil {
		return nil, fmt.Errorf("invalid route ID: %w", err)
	}
	var route Route
	err = coll.FindOne(ctx, bson.M{"_id": objID}).Decode(&route)
	if err == mongo.ErrNoDocuments {
		return nil, errors.New("route not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get route document: %w", err)
	}
	route.ID = objID.Hex()
	return &route, nil
}

// GetAllRoutes retrieves all routes from MongoDB, ordered by submission time
func GetAllRoutes(ctx context.Context) ([]Route, error) {
	coll := mongoDB.Collection(routesCollection)
	opts := options.Find().SetSort(bson.D{{Key: "submittedAt", Value: -1}})
	cursor, err := coll.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("error finding routes: %w", err)
	}
	defer cursor.Close(ctx)

	var routes []Route
	for cursor.Next(ctx) {
		var route Route
		if err := cursor.Decode(&route); err != nil {
			return nil, fmt.Errorf("error decoding route: %w", err)
		}
		if oid, ok := cursor.Current.Lookup("_id").ObjectIDOK(); ok {
			route.ID = oid.Hex()
		}
		routes = append(routes, route)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}
	return routes, nil
}

// GetUserRoutes retrieves routes submitted by a specific user from MongoDB
func GetUserRoutes(ctx context.Context, userID string) ([]Route, error) {
	coll := mongoDB.Collection(routesCollection)
	filter := bson.M{"submittedByUserID": userID}
	opts := options.Find().SetSort(bson.D{{Key: "submittedAt", Value: -1}})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("error finding user routes: %w", err)
	}
	defer cursor.Close(ctx)

	var routes []Route
	for cursor.Next(ctx) {
		var route Route
		if err := cursor.Decode(&route); err != nil {
			return nil, fmt.Errorf("error decoding route: %w", err)
		}
		if oid, ok := cursor.Current.Lookup("_id").ObjectIDOK(); ok {
			route.ID = oid.Hex()
		}
		routes = append(routes, route)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}
	return routes, nil
}

// DeleteRoute deletes a route document from MongoDB
func DeleteRoute(ctx context.Context, routeID string) error {
	coll := mongoDB.Collection(routesCollection)
	objID, err := primitive.ObjectIDFromHex(routeID)
	if err != nil {
		return fmt.Errorf("invalid route ID: %w", err)
	}
	_, err = coll.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		return fmt.Errorf("failed to delete route document with ID %s: %w", routeID, err)
	}
	return nil
}
