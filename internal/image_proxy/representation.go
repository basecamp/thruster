package image_proxy

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	decryptionKeyPurpose = "thruster:active_storage:representation"
)

type Representation struct {
	Transformations *map[string]interface{} `json:"transformations"`
	Preview         *PreviewParams `json:"preview"`
	Filename        string                 `json:"filename"`
	ContentType     string                 `json:"content_type"`
	ByteSize        int64                  `json:"byte_size"`
	Checksum        string                 `json:"checksum"`
	DownloadURL     string                 `json:"download_url"`
}

type PreviewParams struct {
	Command   string   `json:"command"`
	Arguments []string `json:"arguments"`
}

type ProcessedRepresentation struct {
	Reader      io.Reader
	ContentType string
	Close       func() error
}

func ParseRepresentation(secret, data string) (*Representation, error) {
	encryptedData, err := decodeRepresentationPayload(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode representation payload: %w", err)
	}

	compressedData, err := decryptRepresentationPayload(secret, encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decipher representation payload: %w", err)
	}

	jsonData, err := inflateCompressedData(compressedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress representation payload: %w", err)
	}

	var rep Representation
	if err := json.Unmarshal(jsonData, &rep); err != nil {
		return nil, fmt.Errorf("failed to parse representation payload JSON: %w", err)
	}

	return &rep, nil
}

func decodeRepresentationPayload(data string) ([]byte, error) {
	decodedData, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	return decodedData, nil
}

func decryptRepresentationPayload(secret string, encryptedData []byte) ([]byte, error) {
	if len(encryptedData) < 28 {
		return nil, fmt.Errorf("encrypted data too short")
	}

	iv := encryptedData[0:12]
	authTag := encryptedData[12:28]
	ciphertext := encryptedData[28:]
	key := generateDecryptionKey(secret)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	ciphertextWithTag := append(ciphertext, authTag...)

	data, err := aesgcm.Open(nil, iv, ciphertextWithTag, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid or tampered data: %w", err)
	}

	return data, nil
}

func generateDecryptionKey(secret string) []byte {
	return pbkdf2.Key([]byte(secret), []byte(decryptionKeyPurpose), 65536, 32, sha256.New)
}

func inflateCompressedData(compressedData []byte) ([]byte, error) {
	zlibReader, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, fmt.Errorf("failed to create zlib reader: %w", err)
	}
	defer zlibReader.Close()

	data, err := io.ReadAll(zlibReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	return data, nil
}

func (r *Representation) Process() (*ProcessedRepresentation, error) {
	// TODO: Implement actual processing logic
	// This should:
	// 1. Download the file from DownloadURL
	// 2. Apply transformations if any
	// 3. Generate preview if needed
	// 4. Return the processed result
	return nil, fmt.Errorf("not implemented yet")
}
