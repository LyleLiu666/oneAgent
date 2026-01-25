package main

import (
	"log"
)

func main() {
	log.Fatalf("migrate is deprecated: Postgres/DATABASE_URL is not supported in local tool mode")
}
