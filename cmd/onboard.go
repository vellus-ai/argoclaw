package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/vellus-ai/argoclaw/internal/config"
)

var nonInteractive bool

type nonInteractiveOpts struct {
	configPath string
	envPath    string
	skipDB     bool // only for unit tests
}

func onboardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "onboard",
		Short: "Quick setup — configure database, generate keys, run migrations",
		Run: func(cmd *cobra.Command, args []string) {
			runOnboard()
		},
	}
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false,
		"Skip all interactive prompts; read all inputs from environment variables.\n"+
			"Required: ARGOCLAW_POSTGRES_DSN\n"+
			"Optional: ARGOCLAW_GATEWAY_TOKEN, ARGOCLAW_ENCRYPTION_KEY (auto-generated if absent)")
	return cmd
}

func runOnboard() {
	if nonInteractive {
		cfgPath := resolveConfigPath()
		cfgDir := filepath.Dir(cfgPath)
		opts := nonInteractiveOpts{
			configPath: cfgPath,
			envPath:    filepath.Join(cfgDir, ".env.local"),
		}
		if err := runOnboardNonInteractiveE(opts); err != nil {
			log.Fatalf("onboard failed: %v", err)
		}
		return
	}

	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║        ArgoClaw — Quick Setup                 ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Println()

	cfgPath := resolveConfigPath()
	cfg := config.Default()

	// Load existing config if present (preserve gateway port, etc.)
	if _, err := os.Stat(cfgPath); err == nil {
		if loaded, err := config.Load(cfgPath); err == nil {
			cfg = loaded
		}
	}

	// ── Step 1: Postgres connection ──
	postgresDSN := os.Getenv("ARGOCLAW_POSTGRES_DSN")
	if postgresDSN == "" {
		postgresDSN = cfg.Database.PostgresDSN
	}
	if postgresDSN == "" {
		fmt.Println("── Database Connection ──")
		fmt.Println("  Enter your PostgreSQL connection details (press Enter for defaults).")
		fmt.Println()

		dsn, err := promptPostgresFields()
		if err != nil {
			fmt.Println("Cancelled.")
			return
		}
		postgresDSN = dsn
	} else {
		fmt.Printf("  Using Postgres DSN from environment\n")
	}

	// ── Step 2: Test connection ──
	fmt.Print("  Testing Postgres connection... ")
	if err := testPostgresConnection(postgresDSN); err != nil {
		fmt.Println("FAILED")
		fmt.Printf("  Error: %v\n", err)
		fmt.Println("  Please check your DSN and try again: ./argoclaw onboard")
		return
	}
	fmt.Println("OK")

	// ── Step 3: Generate keys ──
	gatewayToken := os.Getenv("ARGOCLAW_GATEWAY_TOKEN")
	if gatewayToken == "" {
		gatewayToken = cfg.Gateway.Token
	}
	generatedToken := false
	if gatewayToken == "" {
		gatewayToken = onboardGenerateToken(16)
		generatedToken = true
	}

	encryptionKey := os.Getenv("ARGOCLAW_ENCRYPTION_KEY")
	generatedEncKey := false
	if encryptionKey == "" {
		encryptionKey = onboardGenerateToken(32)
		generatedEncKey = true
	}
	os.Setenv("ARGOCLAW_ENCRYPTION_KEY", encryptionKey)

	// ── Step 4: Migrations ──
	fmt.Print("  Running migrations... ")
	m, err := newMigrator(postgresDSN)
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		fmt.Println("  You can run it manually later: ./argoclaw migrate up")
	} else {
		if err := m.Up(); err != nil && err.Error() != "no change" {
			fmt.Printf("FAILED: %v\n", err)
			fmt.Println("  You can run it manually later: ./argoclaw migrate up")
		} else {
			v, _, _ := m.Version()
			fmt.Printf("OK (version: %d)\n", v)
		}
		m.Close()
	}

	// ── Step 5: Seed placeholder providers for UI ──
	fmt.Print("  Seeding placeholder providers... ")
	if err := seedOnboardPlaceholders(postgresDSN); err != nil {
		fmt.Printf("warning: %v\n", err)
	} else {
		fmt.Println("OK")
	}

	// ── Step 6: Save config ──
	if cfg.Gateway.Host == "" {
		cfg.Gateway.Host = "0.0.0.0"
	}
	if cfg.Gateway.Port == 0 {
		cfg.Gateway.Port = 18790
	}
	cfg.Database.PostgresDSN = "" // secrets go in .env.local, not config
	cfg.Gateway.Token = ""       // secrets go in .env.local, not config

	if err := config.Save(cfgPath, cfg); err != nil {
		fmt.Printf("  Error saving config: %v\n", err)
		os.Exit(1)
	}

	// ── Step 7: Save .env.local ──
	envPath := filepath.Join(filepath.Dir(cfgPath), ".env.local")
	onboardWriteEnvFile(envPath, postgresDSN, gatewayToken, encryptionKey)

	// ── Summary ──
	port := strconv.Itoa(cfg.Gateway.Port)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║           Setup Complete!                    ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Println()

	if generatedToken || generatedEncKey {
		fmt.Println("── Generated Secrets (shown once, saved to .env.local) ──")
		fmt.Println()
		if generatedToken {
			fmt.Printf("  Gateway Token:   %s\n", gatewayToken)
		}
		if generatedEncKey {
			fmt.Printf("  Encryption Key:  %s\n", encryptionKey)
		}
		fmt.Println()
		fmt.Println("  ⚠  These keys are shown only once. They are saved in:")
		fmt.Printf("    → %s\n", envPath)
		fmt.Println()
	}

	fmt.Println("── Files ──")
	fmt.Println()
	fmt.Printf("  Config:    %s  (gateway host/port, no secrets)\n", cfgPath)
	fmt.Printf("  Secrets:   %s  (ARGOCLAW_POSTGRES_DSN, ARGOCLAW_GATEWAY_TOKEN, ARGOCLAW_ENCRYPTION_KEY)\n", envPath)
	fmt.Println()

	fmt.Println("── Next Steps ──")
	fmt.Println()
	fmt.Println("  1. Start the gateway:")
	fmt.Printf("     source %s && ./argoclaw\n", envPath)
	fmt.Println()
	fmt.Println("  2. Open the dashboard to complete setup:")
	fmt.Printf("     http://localhost:%s\n", port)
	fmt.Println()
	fmt.Println("     The setup wizard will guide you through:")
	fmt.Println("     → Provider & API key configuration")
	fmt.Println("     → Model selection & verification")
	fmt.Println("     → Agent creation")
	fmt.Println("     → Channel setup (optional)")
	fmt.Println()
	fmt.Println("  Tip: Use 'goclaw auth anthropic' to authenticate with an")
	fmt.Println("  Anthropic setup token (from 'claude setup-token').")
	fmt.Println()
}

// runOnboardNonInteractiveE executes the onboard flow without any interactive prompts.
// All inputs are read from environment variables. Returns an error instead of calling log.Fatal
// to allow unit testing.
func runOnboardNonInteractiveE(opts nonInteractiveOpts) error {
	dsn := os.Getenv("ARGOCLAW_POSTGRES_DSN")
	if dsn == "" {
		return fmt.Errorf("--non-interactive requires ARGOCLAW_POSTGRES_DSN to be set")
	}

	token := os.Getenv("ARGOCLAW_GATEWAY_TOKEN")
	if token == "" {
		token = onboardGenerateToken(16)
	}

	encKey := os.Getenv("ARGOCLAW_ENCRYPTION_KEY")
	if encKey == "" {
		encKey = onboardGenerateToken(32)
	}

	if !opts.skipDB {
		if err := testPostgresConnection(dsn); err != nil {
			return fmt.Errorf("postgres connection failed: %w", err)
		}

		m, err := newMigrator(dsn)
		if err != nil {
			return fmt.Errorf("create migrator: %w", err)
		}
		if err := m.Up(); err != nil && err.Error() != "no change" {
			return fmt.Errorf("run migrations: %w", err)
		}

		if err := seedOnboardPlaceholders(dsn); err != nil {
			return fmt.Errorf("seed providers: %w", err)
		}
	}

	// Build minimal config (no secrets stored in config file)
	cfg := config.Default()
	if err := config.Save(opts.configPath, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	onboardWriteEnvFile(opts.envPath, dsn, token, encKey)

	fmt.Printf("✓ ArgoClaw onboard complete (non-interactive)\n")
	fmt.Printf("  Config:  %s\n", opts.configPath)
	fmt.Printf("  Secrets: %s\n", opts.envPath)
	return nil
}
