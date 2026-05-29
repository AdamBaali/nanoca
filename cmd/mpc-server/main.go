package main

import (
	"crypto"
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
	caCertPEM := os.Getenv("CA_CERT_PEM")
	caKeyPEM := os.Getenv("CA_KEY_PEM")
	baseURL := envOr("BASE_URL", "https://cert.mpc.ad")
	badgerDir := envOr("BADGER_DIR", "/data/badger")

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// CA cert/key can be supplied either as raw PEM via CA_CERT_PEM/CA_KEY_PEM
	// (handy for platforms like Render that expose secrets as env vars, not
	// mounted files) or as file paths via CA_CERT/CA_KEY. PEM takes precedence.
	caCert, err := loadCert(caCertPath, caCertPEM)
	if err != nil {
		return fmt.Errorf("load CA cert: %w", err)
	}

	signer, err := loadSigner(caKeyPath, caKeyPEM)
	if err != nil {
		return fmt.Errorf("load CA key: %w", err)
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

// loadCert parses an X.509 certificate from raw PEM (certPEM) if provided,
// otherwise from the PEM file at path.
func loadCert(path, certPEM string) (*x509.Certificate, error) {
	data := []byte(certPEM)
	src := "CA_CERT_PEM"
	if certPEM == "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		data = b
		src = path
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in %s", src)
	}
	return x509.ParseCertificate(block.Bytes)
}

// loadSigner returns a crypto.Signer from raw PEM (keyPEM) if provided,
// otherwise it delegates to the file signer for the key at path.
func loadSigner(path, keyPEM string) (crypto.Signer, error) {
	if keyPEM != "" {
		return parseSigner([]byte(keyPEM))
	}
	return filesigner.LoadSigner(path)
}

// parseSigner parses a private key from PEM bytes. Unlike the library's file
// signer (PKCS #8 only), this also accepts PKCS #1 RSA and SEC1 EC keys so that
// whatever the operator pastes into CA_KEY_PEM just works.
func parseSigner(keyData []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in CA_KEY_PEM")
	}
	switch block.Type {
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS#8 key: %w", err)
		}
		signer, ok := key.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("unsupported PKCS#8 key type: %T", key)
		}
		return signer, nil
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS#1 RSA key: %w", err)
		}
		return key, nil
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse EC key: %w", err)
		}
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
