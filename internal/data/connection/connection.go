package connection

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"wameter/internal/data/config"
	"wameter/internal/data/elastic"
	"wameter/internal/data/meili"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

// Connections struct to hold all database connections and clients
type Connections struct {
	DBM    *DBManager
	RC     *redis.Client
	MS     *meili.Client
	ES     *elastic.Client
	MGM    *MongoManager
	Neo    neo4j.DriverWithContext
	RMQ    *amqp.Connection
	KFK    *kafka.Conn
	closed bool
	mu     sync.Mutex
}

// New creates new Connections
func New(cfg *config.Config) (*Connections, error) {
	c := &Connections{}
	var err error

	if cfg.Database != nil && cfg.Database.Master != nil && cfg.Database.Master.Source != "" {
		c.DBM, err = NewDBManager(cfg.Database)
		if err != nil {
			return nil, err
		}
	}

	if cfg.Redis != nil && cfg.Redis.Addr != "" {
		c.RC, err = newRedis(cfg.Redis)
		if err != nil {
			return nil, err
		}
	}

	if cfg.Meilisearch != nil && cfg.Meilisearch.Host != "" {
		c.MS, err = newMeilisearch(cfg.Meilisearch)
		if err != nil {
			return nil, err
		}
	}

	if cfg.Elasticsearch != nil && len(cfg.Elasticsearch.Addresses) > 0 {
		c.ES, err = newElasticsearch(cfg.Elasticsearch)
		if err != nil {
			return nil, err
		}
	}

	if cfg.MongoDB != nil && cfg.MongoDB.Master.URI != "" {
		c.MGM, err = NewMongoManager(cfg.MongoDB)
		if err != nil {
			return nil, err
		}
	}

	if cfg.Neo4j != nil && cfg.Neo4j.URI != "" {
		c.Neo, err = newNeo4j(cfg.Neo4j)
		if err != nil {
			return nil, err
		}
	}

	if cfg.RabbitMQ != nil && cfg.RabbitMQ.URL != "" {
		c.RMQ, err = newRabbitMQ(cfg.RabbitMQ)
		if err != nil {
			return nil, err
		}
	}

	if cfg.Kafka != nil && cfg.Kafka.Brokers != nil && len(cfg.Kafka.Brokers) > 0 {
		c.KFK, err = newKafka(cfg.Kafka)
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

// Close closes all data connections
func (d *Connections) Close() (errs []error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if already closed
	if d.closed {
		return nil
	}

	// Close Redis client if connected
	if d.RC != nil {
		if err := d.pingRedis(context.Background()); err == nil {
			if err := d.RC.Close(); err != nil {
				errs = append(errs, errors.New("redis close error: "+err.Error()))
			}
		}
		d.RC = nil
	}

	// Close database connections if connected
	if d.DBM != nil {
		if err := d.DBM.Close(); err != nil {
			errs = append(errs, errors.New("database close error: "+err.Error()))
		}
		d.DBM = nil
	}

	// Disconnect MongoDB client if connected
	if d.MGM != nil {
		if err := d.MGM.Close(context.Background()); err != nil {
			errs = append(errs, errors.New("mongodb close error: "+err.Error()))
		}
		d.MGM = nil
	}

	// Close Neo4j client if connected
	if d.Neo != nil {
		if err := d.Neo.Close(context.Background()); err != nil {
			errs = append(errs, errors.New("neo4j close error: "+err.Error()))
		}
		d.Neo = nil
	}

	// Close RabbitMQ client if connected
	if d.RMQ != nil {
		if !d.RMQ.IsClosed() {
			if err := d.RMQ.Close(); err != nil {
				errs = append(errs, errors.New("rabbitmq close error: "+err.Error()))
			}
		}
		d.RMQ = nil
	}

	// Close Kafka client if connected
	if d.KFK != nil {
		if err := d.pingKafka(); err == nil {
			if err := d.KFK.Close(); err != nil {
				errs = append(errs, errors.New("kafka close error: "+err.Error()))
			}
		}
		d.KFK = nil
	}

	d.closed = true

	return errs
}

// Ping checks all database connections
func (d *Connections) Ping(ctx context.Context) error {
	if d.DBM != nil {
		return d.DBM.Health(ctx)
	}
	return nil
}

// DB returns the master database connection for write operations
func (d *Connections) DB() *sql.DB {
	if d.DBM == nil {
		return nil
	}
	return d.DBM.Master()
}

// DBRead returns slave database connection for read operations
func (d *Connections) DBRead() (*sql.DB, error) {
	if d.DBM == nil {
		return nil, errors.New("database manager is nil")
	}
	return d.DBM.Slave()
}

// pingRedis checks if Redis connection is alive
func (d *Connections) pingRedis(ctx context.Context) error {
	if d.RC == nil {
		return errors.New("redis client is nil")
	}
	return d.RC.Ping(ctx).Err()
}

// pingKafka checks if Kafka connection is alive
func (d *Connections) pingKafka() error {
	if d.KFK == nil {
		return errors.New("kafka connection is nil")
	}

	// Try to read connection properties as connection check
	_, err := d.KFK.Controller()
	return err
}
