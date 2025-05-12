package main

import (
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/client"
)

/*
To start a MariaDB instance for testing, run:
  docker run --name mariadb_db --rm \
    -e MARIADB_DATABASE=meroxadb \
    -e MARIADB_USER=meroxauser \
    -e MARIADB_PASSWORD=meroxapass \
    -e MARIADB_ROOT_PASSWORD=meroxaadmin \
    -p 3307:3306 \
    -d mariadb:11.4 \
    --binlog-format=ROW \
    --log-bin=mysql-bin \
    --server-id=1
*/

func TestBug(t *testing.T) {
	cfg := canal.NewDefaultConfig()
	cfg.User = "root"
	cfg.Password = "meroxaadmin"
	cfg.Addr = "127.0.0.1:3307"
	cfg.Dump.ExecutionPath = ""
	cfg.ParseTime = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	{
		conn, err := client.Connect(cfg.Addr, cfg.User, cfg.Password, "")
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}

		// Ensure the test database exists
		_, err = conn.Execute("CREATE DATABASE IF NOT EXISTS test_db")
		if err != nil {
			t.Fatalf("failed to create database: %v", err)
		}

		conn.Close()
	}

	{
		c, err := canal.NewCanal(cfg)
		if err != nil {
			panic(err)
		}

		c.SetEventHandler(&LoggingEventHandler{})

		masterPos, err := c.GetMasterPos()
		if err != nil {
			panic(err)
		}

		go func() {
			if err := c.RunFrom(masterPos); err != nil {
				panic(err)
			}
		}()
	}

	{
		// let some margin for canal to start
		time.Sleep(100 * time.Millisecond)

		conn, err := client.Connect(cfg.Addr, cfg.User, cfg.Password, "test_db")
		if err != nil {
			t.Fatalf("failed to connect to test_db: %v", err)
		}

		_, err = conn.Execute("CREATE TABLE IF NOT EXISTS test_table (id INT PRIMARY KEY)")
		if err != nil {
			t.Fatalf("failed to create table: %v", err)
		}
		_, err = conn.Execute("TRUNCATE TABLE test_table")
		if err != nil {
			t.Fatalf("failed to truncate table: %v", err)
		}

		_, err = conn.Execute("INSERT INTO test_table (id) VALUES (1)")
		if err != nil {
			t.Fatalf("failed to insert row: %v", err)
		}
		_, err = conn.Execute("DELETE FROM test_table WHERE id = 1")
		if err != nil {
			t.Fatalf("failed to delete row: %v", err)
		}
	}

	time.Sleep(10 * time.Second)
}

type LoggingEventHandler struct {
	canal.DummyEventHandler
}

func (h *LoggingEventHandler) OnRow(e *canal.RowsEvent) error {
	fmt.Println("e.Header.LogPos", e.Header.LogPos)
	return nil
}
