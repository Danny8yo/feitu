package dependencies

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestKafkaRejectsOpenTCPPortWithoutMetadata(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			defer conn.Close()
			time.Sleep(150 * time.Millisecond)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := checkKafka(ctx, []string{listener.Addr().String()}); err == nil {
		t.Fatal("bare TCP socket passed Kafka check")
	}
	if time.Since(start) > time.Second {
		t.Fatal("Kafka check ignored deadline")
	}
}

func TestInvalidMySQLConfigBecomesFailedCheck(t *testing.T) {
	t.Setenv("MYSQL_DSN", "invalid-dsn")
	checks, closeClients := New()
	defer closeClients()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := checks[0].Run(ctx); err == nil {
		t.Fatal("invalid DSN reported healthy")
	}
}
