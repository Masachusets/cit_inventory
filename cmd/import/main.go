package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Masachusets/stock_wise/internal/importer"
)

func main() {
	dbURL := flag.String("db", "postgres://stockwise:stockwise@localhost:5432/stockwise", "PostgreSQL connection string")
	excelDir := flag.String("excel", "excel", "Path to directory with Excel files")
	flag.Parse()

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "unable to ping database: %v\n", err)
		os.Exit(1)
	}

	log.Println("Connected to PostgreSQL")

	imp := importer.New(pool, *excelDir)
	if err := imp.Run(ctx); err != nil {
		log.Fatalf("import failed: %v", err)
	}
}
