package bilibili

import (
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const defaultCredentialsDir = "./data/credentials"

var defaultEncryptionKey = []byte("a463b25e5f694b8f85bd805f272723e8")

// Options controls optional service behavior.
type Options struct {
	CredentialsDir string
	EncryptionKey  []byte
}

// Service owns Bilibili account persistence, credential files, and secret handling.
type Service struct {
	db             *gorm.DB
	logger         *zap.Logger
	encryptionKey  []byte
	credentialsDir string
}

// NewService creates a reusable Bilibili account module.
func NewService(db *gorm.DB, logger *zap.Logger, options Options) *Service {
	credentialsDir := defaultCredentialsDir
	if options.CredentialsDir != "" {
		credentialsDir = options.CredentialsDir
	}

	encryptionKey := defaultEncryptionKey
	if len(options.EncryptionKey) > 0 {
		encryptionKey = append([]byte(nil), options.EncryptionKey...)
	}

	return &Service{
		db:             db,
		logger:         logger,
		encryptionKey:  encryptionKey,
		credentialsDir: credentialsDir,
	}
}