package db

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error
	for i := range 10 {
		pool, err = pgxpool.New(ctx, dsn)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				return pool, nil
			}
		}
		wait := time.Duration(i+1) * 500 * time.Millisecond
		time.Sleep(wait)
	}
	return nil, fmt.Errorf("db connect failed after retries: %w", err)
}

// RunMigrations executes embedded SQL files in lexicographic order.
// Files must be named V1_*.sql, V2_*.sql, etc.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, fs embed.FS, dir string) error {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return err
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		sql, readErr := fs.ReadFile(dir + "/" + f)
		if readErr != nil {
			return fmt.Errorf("read migration %s: %w", f, readErr)
		}
		if _, execErr := pool.Exec(ctx, string(sql)); execErr != nil {
			return fmt.Errorf("exec migration %s: %w", f, execErr)
		}
	}
	return nil
}
