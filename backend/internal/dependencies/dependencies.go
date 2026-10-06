package dependencies

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"feitu/backend/internal/health"
	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

const Timeout = 3 * time.Second

func Env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// New does not require dependencies to be online: liveness remains available.
func New() ([]health.Check, func()) {
	dsn := Env("MYSQL_DSN", "feitu@tcp(127.0.0.1:13306)/feitu")
	cfg, configErr := mysql.ParseDSN(dsn)
	var db *sql.DB
	if configErr == nil {
		cfg.Timeout = Timeout
		cfg.ReadTimeout = Timeout
		cfg.WriteTimeout = Timeout
		cfg.Logger = &mysql.NopLogger{}
		c, err := mysql.NewConnector(cfg)
		configErr = err
		if err == nil {
			db = sql.OpenDB(c)
			db.SetMaxOpenConns(4)
			db.SetMaxIdleConns(4)
			db.SetConnMaxLifetime(3 * time.Minute)
		}
	}
	rc := redis.NewClient(&redis.Options{Addr: Env("REDIS_ADDR", "127.0.0.1:16379"), Password: os.Getenv("REDIS_PASSWORD"), DialTimeout: Timeout, ReadTimeout: Timeout, WriteTimeout: Timeout, ContextTimeoutEnabled: true, MaxRetries: -1, PoolSize: 4})
	brokers := strings.Split(Env("KAFKA_BROKERS", "127.0.0.1:19092"), ",")
	checks := []health.Check{
		{Name: "mysql", Run: func(ctx context.Context) error {
			if configErr != nil {
				return errors.New("invalid mysql configuration")
			}
			return db.PingContext(ctx)
		}},
		{Name: "kafka", Run: func(ctx context.Context) error { return checkKafka(ctx, brokers) }},
		{Name: "redis", Run: func(ctx context.Context) error { return rc.Ping(ctx).Err() }},
	}
	return checks, func() {
		if db != nil {
			_ = db.Close()
		}
		_ = rc.Close()
	}
}

func checkKafka(ctx context.Context, addresses []string) error {
	dialer := &kafka.Dialer{Timeout: Timeout}
	for _, address := range addresses {
		conn, err := dialer.DialContext(ctx, "tcp", strings.TrimSpace(address))
		if err != nil {
			continue
		}
		deadline := time.Now().Add(Timeout)
		if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
			deadline = d
		}
		_ = conn.SetDeadline(deadline)
		brokers, err := conn.Brokers()
		_ = conn.Close()
		if err != nil || len(brokers) == 0 {
			continue
		}
		// Verify metadata's advertised addresses too, catching a wrong Kafka listener.
		for _, broker := range brokers {
			target := net.JoinHostPort(broker.Host, strconv.Itoa(broker.Port))
			advertised, err := dialer.DialContext(ctx, "tcp", target)
			if err != nil {
				return errors.New("advertised broker unavailable")
			}
			_ = advertised.SetDeadline(deadline)
			metadata, err := advertised.Brokers()
			_ = advertised.Close()
			if err != nil || len(metadata) == 0 {
				return errors.New("advertised broker metadata unavailable")
			}
		}
		return nil
	}
	return errors.New("kafka metadata unavailable")
}
