package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	storagepostgres "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/storage/postgres"
	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
)

const commandTimeout = 15 * time.Second

func main() {
	var (
		databaseURL  = flag.String("database-url", strings.TrimSpace(os.Getenv("DATABASE_URL")), "Postgres connection string")
		phoneNumber  = flag.String("phone", "", "Phone number to register, e.g. 5511999999999")
		producerName = flag.String("producer", "", "Producer or owner name")
		farmName     = flag.String("farm", "", "Farm or business name")
	)
	flag.Parse()

	if strings.TrimSpace(*databaseURL) == "" {
		log.Fatal("missing database connection: provide --database-url or DATABASE_URL")
	}
	if strings.TrimSpace(*phoneNumber) == "" {
		log.Fatal("missing phone number: provide --phone")
	}
	if strings.TrimSpace(*producerName) == "" {
		log.Fatal("missing producer name: provide --producer")
	}
	if strings.TrimSpace(*farmName) == "" {
		log.Fatal("missing farm name: provide --farm")
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	database, err := storagepostgres.OpenDatabase(ctx, strings.TrimSpace(*databaseURL))
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()

	if err := storagepostgres.EnsureSchema(ctx, database); err != nil {
		log.Fatalf("ensure schema: %v", err)
	}

	repository := storagepostgres.NewFarmRegistrationRepository(database)
	membership, err := repository.CreateInitialRegistration(
		ctx,
		strings.TrimSpace(*phoneNumber),
		strings.TrimSpace(*producerName),
		strings.TrimSpace(*farmName),
	)
	if err != nil {
		log.Fatalf("create registration: %v", err)
	}

	fmt.Printf("registered phone=%s farm_id=%s farm=%s role=%s\n",
		domain.NormalizePhoneNumber(membership.PhoneNumber),
		membership.FarmID,
		membership.FarmName,
		membership.Role,
	)
}
