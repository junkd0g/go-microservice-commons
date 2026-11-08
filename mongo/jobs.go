// Package mongo supplies optional MongoDB integrations for commons.
package mongo

import (
	"context"
	"time"

	"github.com/junkd0g/go-microservice-commons/jobrunner"
	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	mongoSet   = "$set"
	mongoID    = "_id"
	mongoOwner = "owner"
	mongoUntil = "until"
)

// Mongo stores job leases and run records in MongoDB.
type Mongo struct{ leases, runs *mongodriver.Collection }

// NewJobStore selects the lease and run collections.
var _ jobrunner.Store = (*Mongo)(nil)

func NewJobStore(db *mongodriver.Database) *Mongo {
	return &Mongo{db.Collection("scheduled_job_leases"), db.Collection("scheduled_job_runs")}
}

// Ensure creates the run expiry index.
func (m *Mongo) Ensure(ctx context.Context) error {
	_, err := m.runs.Indexes().CreateOne(ctx, mongodriver.IndexModel{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)})
	return err
}

// Acquire claims an absent or expired lease.
func (m *Mongo) Acquire(ctx context.Context, key, owner string, ttl time.Duration) (bool, error) {
	now := time.Now().UTC()
	_, err := m.leases.UpdateOne(ctx, bson.M{mongoID: key, mongoUntil: bson.M{"$lte": now}}, bson.M{mongoSet: bson.M{mongoOwner: owner, mongoUntil: now.Add(ttl)}}, options.Update().SetUpsert(true))
	if mongodriver.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}

// Renew extends a lease still owned by this worker.
func (m *Mongo) Renew(ctx context.Context, key, owner string, ttl time.Duration) (bool, error) {
	now := time.Now().UTC()
	r, err := m.leases.UpdateOne(ctx, bson.M{mongoID: key, mongoOwner: owner, mongoUntil: bson.M{"$gt": now}}, bson.M{mongoSet: bson.M{mongoUntil: now.Add(ttl)}})
	if err != nil {
		return false, err
	}
	return r.MatchedCount == 1, nil
}

// Release removes a lease owned by this worker.
func (m *Mongo) Release(ctx context.Context, key, owner string) error {
	_, err := m.leases.DeleteOne(ctx, bson.M{mongoID: key, mongoOwner: owner})
	return err
}

// Begin records an attempt and preserves its business date.
func (m *Mongo) Begin(ctx context.Context, key string, date time.Time) (time.Time, bool, error) {
	var r struct {
		Date      time.Time `bson:"date"`
		Completed bool      `bson:"completed"`
	}
	err := m.runs.FindOneAndUpdate(ctx, bson.M{mongoID: key}, bson.M{"$setOnInsert": bson.M{"date": date, "completed": false}, mongoSet: bson.M{"expires_at": time.Now().Add(30 * 24 * time.Hour)}, "$inc": bson.M{"attempts": 1}}, options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&r)
	return r.Date, r.Completed, err
}

// Finish records completion or failure.
func (m *Mongo) Finish(ctx context.Context, key, message string) error {
	_, err := m.runs.UpdateOne(ctx, bson.M{mongoID: key}, bson.M{mongoSet: bson.M{"completed": message == "", "error": message, "updated_at": time.Now().UTC()}})
	return err
}
