package service

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"time"
)

type SignerService struct {
	privateKeyPath string
	publicKeyPath  string
}

func NewSignerService(keyDir string) *SignerService {
	return &SignerService{
		privateKeyPath: keyDir + "/private.key",
		publicKeyPath:  keyDir + "/public.key",
	}
}

type SignatureInfo struct {
	Hash      string    `json:"hash"`
	Size      int64     `json:"size"`
	SignedAt  time.Time `json:"signed_at"`
	Algorithm string    `json:"algorithm"`
}

func (s *SignerService) SignModule(zipPath string) (*SignatureInfo, error) {
	f, err := os.Open(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("hash zip: %w", err)
	}

	fi, _ := f.Stat()

	return &SignatureInfo{
		Hash:      fmt.Sprintf("%x", h.Sum(nil)),
		Size:      fi.Size(),
		SignedAt:  time.Now(),
		Algorithm: "SHA256",
	}, nil
}

func (s *SignerService) VerifyModule(zipPath, expectedHash string) (bool, error) {
	info, err := s.SignModule(zipPath)
	if err != nil {
		return false, err
	}
	return info.Hash == expectedHash, nil
}

func (s *SignerService) GenerateSignatureManifest(zipPath string) (string, error) {
	info, err := s.SignModule(zipPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`{
  "algorithm": "%s",
  "hash": "%s",
  "size": %d,
  "signed_at": "%s",
  "tool": "ModuForge Signer v1.0"
}`, info.Algorithm, info.Hash, info.Size, info.SignedAt.Format(time.RFC3339)), nil
}
