package main

import (
	"flag"
	"log"
	"os"

	"github.com/kprimice/challenge-week/pkg/scanner"
	"github.com/kprimice/challenge-week/pkg/scanner/types"
)

func main() {
	startFlag := flag.Uint64("start", 96000000, "Block number to start scanning downward from.")
	endFlag := flag.Uint64("end", 103000000, "Block number to stop at (inclusive).")
	marketFlag := flag.String("market", "", "Market ID to filter (optional). If empty, fetch all derivative trades for all markets.")

	flag.Parse()

	ordersFile, err := os.Create("./data/orders.csv")
	if err != nil {
		log.Fatalf("failed to create orders CSV file: %w", err)
	}
	defer ordersFile.Close()

	tradesFile, err := os.Create("./data/liquidations.csv")
	if err != nil {
		log.Fatalf("failed to create trades CSV file: %w", err)
	}
	defer tradesFile.Close()

	cfg := types.Config{
		StartBlock: *startFlag,
		EndBlock:   *endFlag,
		MarketID:   *marketFlag,

		// You can add more fields if needed (like concurrency, pageSize, chain network, etc.)
	}

	if err := scanner.RunScanner(cfg, ordersFile, tradesFile); err != nil {
		log.Fatalf("Scanner error: %v", err)
	}

	log.Println("Done!")
}
