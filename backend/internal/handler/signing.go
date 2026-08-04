package handler

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

type SigningHandler struct {
	db      *sql.DB
	keyDir  string
}

func NewSigningHandler(db *sql.DB, keyDir string) *SigningHandler {
	os.MkdirAll(keyDir, 0700)
	return &SigningHandler{db: db, keyDir: keyDir}
}

func (h *SigningHandler) privateKeyPath() string {
	return filepath.Join(h.keyDir, "module_signing.key")
}

func (h *SigningHandler) publicKeyPath() string {
	return filepath.Join(h.keyDir, "module_signing.pub")
}

// loadOrGenerateKeyPair loads existing RSA key pair or generates a new one
func (h *SigningHandler) loadOrGenerateKeyPair() (*rsa.PrivateKey, error) {
	privPath := h.privateKeyPath()

	// Try to load existing key
	if data, err := os.ReadFile(privPath); err == nil {
		block, _ := pem.Decode(data)
		if block != nil {
			key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err == nil {
				return key, nil
			}
		}
	}

	// Generate new 2048-bit RSA key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate RSA key: %w", err)
	}

	// Save private key
	privFile, err := os.Create(privPath)
	if err != nil {
		return nil, fmt.Errorf("create private key file: %w", err)
	}
	defer privFile.Close()
	pem.Encode(privFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	os.Chmod(privPath, 0600)

	// Save public key
	pubFile, err := os.Create(h.publicKeyPath())
	if err != nil {
		return nil, fmt.Errorf("create public key file: %w", err)
	}
	defer pubFile.Close()
	pubASN1, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	pem.Encode(pubFile, &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})

	return key, nil
}

// loadPublicKey loads the public key from file
func (h *SigningHandler) loadPublicKey() (*rsa.PublicKey, error) {
	data, err := os.ReadFile(h.publicKeyPath())
	if err != nil {
		return nil, fmt.Errorf("public key not found: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return rsaPub, nil
}

// computeModuleHash computes SHA-256 hash of all project files
func (h *SigningHandler) computeModuleHash(projectID string) (string, error) {
	rows, err := h.db.Query(
		`SELECT path, content FROM project_files WHERE project_id=? ORDER BY path`, projectID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	h256 := sha256.New()
	for rows.Next() {
		var path, content string
		if err := rows.Scan(&path, &content); err != nil {
			continue
		}
		h256.Write([]byte(path))
		h256.Write([]byte(content))
	}
	return hex.EncodeToString(h256.Sum(nil)), nil
}

// SignModule signs a module's code
// POST /projects/:id/sign
func (h *SigningHandler) SignModule(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return BadRequest(c, "项目 ID 不能为空")
	}

	// Verify project exists
	var exists int
	err := h.db.QueryRow("SELECT 1 FROM projects WHERE id=? AND deleted_at IS NULL", projectID).Scan(&exists)
	if err != nil {
		return NotFound(c, "项目不存在")
	}

	// Compute file hash
	fileHash, err := h.computeModuleHash(projectID)
	if err != nil {
		return InternalError(c, "计算文件哈希失败: "+err.Error())
	}

	// Load or generate key pair
	privateKey, err := h.loadOrGenerateKeyPair()
	if err != nil {
		return InternalError(c, "密钥生成失败: "+err.Error())
	}

	// Sign the hash
	hashBytes, _ := hex.DecodeString(fileHash)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashBytes)
	if err != nil {
		return InternalError(c, "签名失败: "+err.Error())
	}
	signatureHex := hex.EncodeToString(signature)

	// Read public key for storage
	pubData, err := os.ReadFile(h.publicKeyPath())
	if err != nil {
		return InternalError(c, "读取公钥失败: "+err.Error())
	}
	publicKeyPEM := string(pubData)

	// Save to database
	_, err = h.db.Exec(`
		INSERT INTO module_signatures (module_id, public_key, signature, file_hash, signed_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(module_id) DO UPDATE SET
			public_key=excluded.public_key,
			signature=excluded.signature,
			file_hash=excluded.file_hash,
			signed_at=excluded.signed_at`,
		projectID, publicKeyPEM, signatureHex, fileHash, time.Now().Format(time.RFC3339))
	if err != nil {
		return InternalError(c, "保存签名失败: "+err.Error())
	}

	// Return fingerprint (SHA-256 of public key)
	pubKey, _ := h.loadPublicKey()
	pubASN1, _ := x509.MarshalPKIXPublicKey(pubKey)
	fp := sha256.Sum256(pubASN1)

	return c.JSON(fiber.Map{
		"module_id":    projectID,
		"file_hash":    fileHash,
		"signature":    signatureHex,
		"fingerprint":  hex.EncodeToString(fp[:]),
		"signed_at":    time.Now().Format(time.RFC3339),
		"algorithm":    "RSA-2048-SHA256",
	})
}

// VerifyModule verifies a module's signature
// POST /projects/:id/verify
func (h *SigningHandler) VerifyModule(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return BadRequest(c, "项目 ID 不能为空")
	}

	// Get stored signature
	var storedHash, storedSig, storedPub string
	err := h.db.QueryRow(
		"SELECT file_hash, signature, public_key FROM module_signatures WHERE module_id=?",
		projectID).Scan(&storedHash, &storedSig, &storedPub)
	if err == sql.ErrNoRows {
		return c.JSON(fiber.Map{
			"valid":  false,
			"signed": false,
			"error":  "模块未签名",
		})
	}
	if err != nil {
		return InternalError(c, "查询签名失败: "+err.Error())
	}

	// Recompute current file hash
	currentHash, err := h.computeModuleHash(projectID)
	if err != nil {
		return InternalError(c, "计算文件哈希失败: "+err.Error())
	}

	// Check if files have changed
	if currentHash != storedHash {
		return c.JSON(fiber.Map{
			"valid":           false,
			"signed":          true,
			"error":           "文件已被修改，签名无效",
			"stored_hash":     storedHash,
			"current_hash":    currentHash,
		})
	}

	// Verify signature with stored public key
	block, _ := pem.Decode([]byte(storedPub))
	if block == nil {
		return c.JSON(fiber.Map{"valid": false, "signed": true, "error": "公钥格式无效"})
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return c.JSON(fiber.Map{"valid": false, "signed": true, "error": "解析公钥失败"})
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return c.JSON(fiber.Map{"valid": false, "signed": true, "error": "非 RSA 公钥"})
	}

	sigBytes, _ := hex.DecodeString(storedSig)
	hashBytes, _ := hex.DecodeString(currentHash)
	err = rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, hashBytes, sigBytes)

	valid := err == nil
	msg := "签名验证通过"
	if !valid {
		msg = "签名验证失败: " + err.Error()
	}

	return c.JSON(fiber.Map{
		"valid":      valid,
		"signed":     true,
		"file_hash":  currentHash,
		"message":    msg,
	})
}

// GetSignatureInfo returns signature information for a module
// GET /projects/:id/signature
func (h *SigningHandler) GetSignatureInfo(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return BadRequest(c, "项目 ID 不能为空")
	}

	var id int64
	var moduleID, pubKey, sig, fileHash, signedAt string
	err := h.db.QueryRow(
		"SELECT id, module_id, public_key, signature, file_hash, signed_at FROM module_signatures WHERE module_id=?",
		projectID).Scan(&id, &moduleID, &pubKey, &sig, &fileHash, &signedAt)
	if err == sql.ErrNoRows {
		return c.JSON(fiber.Map{
			"signed": false,
		})
	}
	if err != nil {
		return InternalError(c, "查询签名失败: "+err.Error())
	}

	// Compute fingerprint
	block, _ := pem.Decode([]byte(pubKey))
	var fingerprint string
	if block != nil {
		if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
			pubASN1, _ := x509.MarshalPKIXPublicKey(pub)
			fp := sha256.Sum256(pubASN1)
			fingerprint = hex.EncodeToString(fp[:])
		}
	}

	// Truncate public key for display (show only first/last lines)
	pubLines := strings.Split(strings.TrimSpace(pubKey), "\n")
	pubDisplay := pubKey
	if len(pubLines) > 2 {
		pubDisplay = pubLines[0] + "\n..." + pubLines[len(pubLines)-1]
	}

	return c.JSON(fiber.Map{
		"signed":      true,
		"module_id":   moduleID,
		"fingerprint": fingerprint,
		"file_hash":   fileHash,
		"algorithm":   "RSA-2048-SHA256",
		"signed_at":   signedAt,
		"public_key":  pubDisplay,
	})
}
