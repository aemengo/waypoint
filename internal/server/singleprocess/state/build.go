package state

import (
	"math"
	"time"

	"github.com/boltdb/bolt"
	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/go-memdb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/hashicorp/waypoint/internal/server/gen"
)

var buildBucket = []byte("build")

const (
	buildIndexTableName             = "build-index"
	buildIndexIdIndexName           = "id"
	buildIndexCompleteTimeIndexName = "complete-time-by-app"
)

func init() {
	dbBuckets = append(dbBuckets, buildBucket)
	schemas = append(schemas, buildIndexSchema)
}

func buildIndexSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: buildIndexTableName,
		Indexes: map[string]*memdb.IndexSchema{
			buildIndexIdIndexName: &memdb.IndexSchema{
				Name:         buildIndexIdIndexName,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.StringFieldIndex{
					Field: "Id",
				},
			},

			buildIndexCompleteTimeIndexName: &memdb.IndexSchema{
				Name:         buildIndexCompleteTimeIndexName,
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Project",
							Lowercase: true,
						},

						&memdb.StringFieldIndex{
							Field:     "App",
							Lowercase: true,
						},

						&IndexTime{
							Field: "CompleteTime",
						},
					},
				},
			},
		},
	}
}

type buildIndexRecord struct {
	Id           string
	Project      string
	App          string
	CompleteTime time.Time
}

// BuildPut inserts or updates a build record.
func (s *State) BuildPut(update bool, b *pb.Build) error {
	memTxn := s.inmem.Txn(true)
	defer memTxn.Abort()

	err := s.db.Update(func(dbTxn *bolt.Tx) error {
		return s.buildPut(dbTxn, memTxn, update, b)
	})
	if err == nil {
		memTxn.Commit()
	}

	return err
}

// BuildGet gets a build by ID.
func (s *State) BuildGet(id string) (*pb.Build, error) {
	var result pb.Build
	err := s.db.View(func(tx *bolt.Tx) error {
		return dbGet(tx.Bucket(buildBucket), []byte(id), &result)
	})
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// BuildLatest gets the latest build that was completed.
func (s *State) BuildLatest(ref *pb.Ref_Application) (*pb.Build, error) {
	memTxn := s.inmem.Txn(false)
	defer memTxn.Abort()

	iter, err := memTxn.LowerBound(
		buildIndexTableName,
		buildIndexCompleteTimeIndexName,
		ref.Project,
		ref.Application,
		time.Unix(math.MaxInt64, 0),
	)
	if err != nil {
		return nil, err
	}

	raw := iter.Next()
	if raw == nil {
		return nil, nil
	}

	record := raw.(*buildIndexRecord)
	return s.BuildGet(record.Id)
}

func (s *State) buildPut(
	tx *bolt.Tx,
	inmemTxn *memdb.Txn,
	update bool,
	build *pb.Build,
) error {
	id := []byte(build.Id)

	// Get the global bucket and write the value to it.
	b := tx.Bucket(buildBucket)
	if err := dbPut(b, id, build); err != nil {
		return err
	}

	// Create our index value and write that.
	return s.buildPutIndex(inmemTxn, build)
}

func (s *State) buildPutIndex(txn *memdb.Txn, build *pb.Build) error {
	var completeTime time.Time
	if build.Status != nil {
		t, err := ptypes.Timestamp(build.Status.CompleteTime)
		if err != nil {
			return status.Errorf(codes.Internal, "time for build can't be parsed")
		}

		completeTime = t
	}

	return txn.Insert(buildIndexTableName, &buildIndexRecord{
		Id:           build.Id,
		Project:      build.Application.Project,
		App:          build.Application.Application,
		CompleteTime: completeTime,
	})
}
