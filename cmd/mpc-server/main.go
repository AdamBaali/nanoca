package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/brandonweeks/nanoca"
	nullauthorizer "github.com/brandonweeks/nanoca/authorizers/null"
	"github.com/brandonweeks/nanoca/issuers/inprocess"
	filesigner "github.com/brandonweeks/nanoca/signers/file"
	"github.com/brandonweeks/nanoca/storage/badger"
	"github.com/brandonweeks/nanoca/verifiers/apple"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	port := envOr("PORT", "10000")
	caCertPath := envOr("CA_CERT", "/etc/secrets/rootCA.crt")
	caKeyPath := envOr("CA_KEY", "/etc/secrets/rootCA.key")
	baseURL := envOr("BASE_URL", "https://cert.mpc.ad")
	badgerDir := envOr("BADGER_DIR", "/data/badger")

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	caCert, err := loadCert(caCertPath)
	if err != nil {
		return fmt.Errorf("CA cert at %s: %w", caCertPath, err)
	}

	signer, err := filesigner.LoadSigner(caKeyPath)
	if err != nil {
		return fmt.Errorf("CA key at %s: %w", caKeyPath, err)
	}

	storage, err := badger.New(badger.Options{Path: badgerDir})
	if err != nil {
		return fmt.Errorf("badger at %s: %w", badgerDir, err)
	}

	ca, err := nanoca.New(
		logger,
		inprocess.New(caCert, signer),
		nullauthorizer.New(),
		storage,
		baseURL,
		nanoca.WithPrefix("/acme"),
		nanoca.WithVerifier(apple.New(logger)),
	)
	if err != nil {
		return fmt.Errorf("nanoca.New: %w", err)
	}
	defer ca.Close()

	mux := http.NewServeMux()
	mux.Handle("/", ca.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("nanoca listening on :%s, directory: %s/acme/directory", port, baseURL)
	return http.ListenAndServe(":"+port, mux)
}

func loadCert(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in %s", path)
	}
	return x509.ParseCertificate(block.Bytes)
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
