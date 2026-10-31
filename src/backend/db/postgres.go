package db

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourorg/insightflow/src/backend/config"
)

var (
	pool *pgxpool.Pool
	once sync.Once
)

// InitDB initializes the PostgreSQL connection pool, runs migrations and makes the pool
// available to the rest of the application. It is safe to call multiple times; the
// underlying initialization will happen only once.
func InitDB(cfg *config.Config) error {
	var initErr error
	once.Do(func() {
		// Build DSN from configuration.
		dsn := fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s?sslmode=%s",
			cfg.DBUser,
			cfg.DBPassword,
			cfg.DBHost,
			cfg.DBPort,
			cfg.DBName,
			cfg.DBSSLMode,
		)

		// Parse and tweak pool configuration.
		poolCfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			initErr = fmt.Errorf("parsing DB config: %w", err)
			return
		}
		if cfg.DBMaxConns > 0 {
			poolCfg.MaxConns = int32(cfg.DBMaxConns)
		}
		if cfg.DBMinConns > 0 {
			poolCfg.MinConns = int32(cfg.DBMinConns)
		}
		poolCfg.MaxConnLifetime = time.Hour
		poolCfg.HealthCheckPeriod = time.Minute

		// Establish the pool.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pool, err = pgxpool.ConnectConfig(ctx, poolCfg)
		if err != nil {
			initErr = fmt.Errorf("connecting to DB: %w", err)
			return
		}

		// Verify connectivity.
		if err = pool.Ping(ctx); err != nil {
			initErr = fmt.Errorf("ping DB: %w", err)
			return
		}

		// Run schema migrations.
		if err = runMigrations(pool); err != nil {
			initErr = fmt.Errorf("running migrations: %w", err)
			return
		}
	})
	return initErr
}

// GetPool returns the initialized connection pool. The application should have called
// InitDB before invoking this; otherwise the function will log.Fatal.
func GetPool() *pgxpool.Pool {
	if pool == nil {
		log.Fatal("database pool accessed before initialization")
	}
	return pool
}

// Close gracefully shuts down the connection pool. It should be called from main on
// application termination.
func Close() {
	if pool != nil {
		pool.Close()
	}
}

// runMigrations applies all pending migrations located in the ./migrations directory.
// It uses golang-migrate with the pgx driver. Errors are propagated to the caller.
func runMigrations(p *pgxpool.Pool) error {
	// Acquire a single connection for the migration driver.
	conn, err := p.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("acquire connection for migrations: %w", err)
	}
	defer conn.Release()

	// Convert the pgx connection to a *sql.DB compatible driver.
	stdConn, err := pgx.StdlibConn(conn.Conn())
	if err != nil {
		return fmt.Errorf("convert pgx to stdlib connection: %w", err)
	}
	defer stdConn.Close()

	driver, err := postgres.WithInstance(stdConn, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create migrate driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres", driver,
	)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	// Apply all up migrations. ErrNoChange is not a failure.
	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up: %w", err)
	}
	return nil
}

// Exec runs a statement that does not return rows (INSERT, UPDATE, DELETE, etc.).
func Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return GetPool().Exec(ctx, sql, args...)
}

// Query runs a SELECT statement and returns the resulting rows.
func Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return GetPool().Query(ctx, sql, args...)
}

// QueryRow runs a SELECT that is expected to return at most one row.
func QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return GetPool().QueryRow(ctx, sql, args...)
}