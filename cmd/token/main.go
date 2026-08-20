package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/database"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/repository"
	"github.com/irvanmalik48/realm-api/internal/service"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v\nMake sure DATABASE_URL is set in environment or .env file", err)
	}
	defer db.Close()

	repo := repository.NewTokenRepository(db)
	svc := service.NewTokenService(repo, nil, nil)

	subcommand := os.Args[1]
	switch subcommand {
	case "create":
		handleCreate(ctx, svc, os.Args[2:])
	case "list":
		handleList(ctx, svc)
	case "revoke":
		handleRevoke(ctx, svc, os.Args[2:])
	case "inspect":
		handleInspect(ctx, svc, os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Realm API Token Management CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run ./cmd/token <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  create    Generate a new cryptographically secure API token")
	fmt.Println("  list      List all generated API tokens")
	fmt.Println("  revoke    Revoke an active API token by ID")
	fmt.Println("  inspect   Verify a raw token secret against database")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  go run ./cmd/token create -name 'production-app' -scopes 'storage:write,contact:read' -rpm 120 -expires 365d")
	fmt.Println("  go run ./cmd/token list")
	fmt.Println("  go run ./cmd/token revoke -id <token-uuid>")
}

func handleCreate(ctx context.Context, svc service.TokenService, args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	nameFlag := fs.String("name", "admin-token", "Descriptive name for the API token")
	scopesFlag := fs.String("scopes", "*", "Comma-separated list of granted scopes (e.g. storage:write,contact:read)")
	rpmFlag := fs.Int("rpm", 60, "Rate limit in requests per minute")
	expiresFlag := fs.String("expires", "", "Expiration duration (e.g. 30d, 90d, 365d, 24h, 1h)")

	_ = fs.Parse(args)

	var expiresIn time.Duration
	if *expiresFlag != "" {
		d, err := parseDurationCustom(*expiresFlag)
		if err != nil {
			log.Fatalf("Invalid -expires format: %v", err)
		}
		expiresIn = d
	}

	var scopes []string
	for _, s := range strings.Split(*scopesFlag, ",") {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			scopes = append(scopes, trimmed)
		}
	}

	result, err := svc.Create(ctx, model.TokenCreateInput{
		Name:         *nameFlag,
		Scopes:       scopes,
		RateLimitRPM: *rpmFlag,
		ExpiresIn:    expiresIn,
	})
	if err != nil {
		log.Fatalf("Failed to create token: %v", err)
	}

	fmt.Println("------------------------------------------------------------")
	fmt.Println("API Token Created Successfully")
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("ID:          %s\n", result.Token.ID)
	fmt.Printf("Name:        %s\n", result.Token.Name)
	fmt.Printf("Prefix:      %s\n", result.Token.TokenPrefix)
	fmt.Printf("Scopes:      [%s]\n", strings.Join(result.Token.Scopes, ", "))
	fmt.Printf("Rate Limit:  %d requests / min\n", result.Token.RateLimitRPM)
	if result.Token.ExpiresAt != nil {
		fmt.Printf("Expires At:  %s\n", result.Token.ExpiresAt.Format(time.RFC3339))
	} else {
		fmt.Println("Expires At:  Never")
	}
	fmt.Println("------------------------------------------------------------")
	fmt.Println("TOKEN SECRET (Save this now, it cannot be displayed again):")
	fmt.Println()
	fmt.Printf("  %s\n\n", result.Raw)
	fmt.Println("Usage example:")
	fmt.Printf("  curl -H 'Authorization: Bearer %s' https://api.irvanma.eu.org/v1/storage/upload\n", result.Raw)
	fmt.Println("------------------------------------------------------------")
}

func handleList(ctx context.Context, svc service.TokenService) {
	tokens, err := svc.List(ctx)
	if err != nil {
		log.Fatalf("Failed to list tokens: %v", err)
	}

	if len(tokens) == 0 {
		fmt.Println("No API tokens found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tPREFIX\tSCOPES\tRPM\tSTATUS\tEXPIRES\tLAST USED")
	fmt.Fprintln(w, "--\t----\t------\t------\t---\t------\t-------\t---------")

	for _, t := range tokens {
		status := "ACTIVE"
		if t.IsRevoked {
			status = "REVOKED"
		} else if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
			status = "EXPIRED"
		}

		expires := "Never"
		if t.ExpiresAt != nil {
			expires = t.ExpiresAt.Format("2006-01-02 15:04")
		}

		lastUsed := "Never"
		if t.LastUsedAt != nil {
			lastUsed = t.LastUsedAt.Format("2006-01-02 15:04")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
			t.ID,
			t.Name,
			t.TokenPrefix,
			strings.Join(t.Scopes, ","),
			t.RateLimitRPM,
			status,
			expires,
			lastUsed,
		)
	}
	_ = w.Flush()
}

func handleRevoke(ctx context.Context, svc service.TokenService, args []string) {
	fs := flag.NewFlagSet("revoke", flag.ExitOnError)
	idFlag := fs.String("id", "", "UUID of the token to revoke")
	_ = fs.Parse(args)

	if *idFlag == "" {
		log.Fatal("Error: -id parameter is required")
	}

	id, err := uuid.Parse(*idFlag)
	if err != nil {
		log.Fatalf("Invalid UUID format: %v", err)
	}

	if err := svc.Revoke(ctx, id); err != nil {
		log.Fatalf("Failed to revoke token: %v", err)
	}

	fmt.Printf("API Token %s has been revoked successfully.\n", id)
}

func handleInspect(ctx context.Context, svc service.TokenService, args []string) {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	tokenFlag := fs.String("token", "", "Raw API token string to inspect")
	_ = fs.Parse(args)

	if *tokenFlag == "" {
		log.Fatal("Error: -token parameter is required")
	}

	tok, err := svc.Verify(ctx, *tokenFlag)
	if err != nil {
		log.Fatalf("Verification failed: %v", err)
	}

	fmt.Println("------------------------------------------------------------")
	fmt.Println("Token Verification Successful")
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("ID:          %s\n", tok.ID)
	fmt.Printf("Name:        %s\n", tok.Name)
	fmt.Printf("Prefix:      %s\n", tok.TokenPrefix)
	fmt.Printf("Scopes:      [%s]\n", strings.Join(tok.Scopes, ", "))
	fmt.Printf("Rate Limit:  %d RPM\n", tok.RateLimitRPM)
	fmt.Printf("Revoked:     %t\n", tok.IsRevoked)
	if tok.ExpiresAt != nil {
		fmt.Printf("Expires At:  %s\n", tok.ExpiresAt.Format(time.RFC3339))
	} else {
		fmt.Println("Expires At:  Never")
	}
	if tok.LastUsedAt != nil {
		fmt.Printf("Last Used:   %s\n", tok.LastUsedAt.Format(time.RFC3339))
	}
	fmt.Println("------------------------------------------------------------")
}

func parseDurationCustom(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "d") {
		daysStr := strings.TrimSuffix(s, "d")
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
