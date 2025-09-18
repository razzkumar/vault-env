package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/razzkumar/vault-env/pkg/vault"
)

// LoadEnvFile loads a .env file and returns encrypted/plaintext data map
func LoadEnvFile(path string, client *vault.Client, transitMount, keyName string, useEncryption bool) (map[string]interface{}, error) {
	// Use godotenv to parse the .env file
	envMap, err := godotenv.Read(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read .env file: %w", err)
	}

	data := make(map[string]interface{})

	for key, value := range envMap {
		if useEncryption {
			ciphertext, err := client.TransitEncrypt(transitMount, keyName, []byte(value))
			if err != nil {
				return nil, fmt.Errorf("encrypt %s: %w", key, err)
			}
			data[key] = ciphertext
		} else {
			data[key] = value
		}
	}

	return data, nil
}

// LoadFileAsBase64 reads a file and encodes it as base64
func LoadFileAsBase64(path string, client *vault.Client, transitMount, keyName string, useEncryption bool) (map[string]interface{}, error) {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	base64Content := base64.StdEncoding.EncodeToString(fileContent)

	if useEncryption {
		ciphertext, err := client.TransitEncrypt(transitMount, keyName, []byte(base64Content))
		if err != nil {
			return nil, fmt.Errorf("encrypt file content: %w", err)
		}
		return map[string]interface{}{"ciphertext": ciphertext}, nil
	}

	return map[string]interface{}{"value": base64Content}, nil
}

// IsEncryptedSingleValue checks if data contains a single encrypted value
func IsEncryptedSingleValue(data map[string]interface{}) bool {
	if len(data) != 1 {
		return false
	}
	ciphertext, ok := data["ciphertext"].(string)
	return ok && strings.HasPrefix(ciphertext, "vault:v")
}

// IsPlaintextSingleValue checks if data contains a single plaintext value
func IsPlaintextSingleValue(data map[string]interface{}) bool {
	if len(data) != 1 {
		return false
	}
	_, hasValue := data["value"]
	return hasValue
}

// IsEncryptedMultiValue checks if data contains multiple encrypted values
func IsEncryptedMultiValue(data map[string]interface{}) bool {
	if len(data) == 0 {
		return false
	}
	
	for _, v := range data {
		if str, ok := v.(string); ok && strings.HasPrefix(str, "vault:v") {
			return true
		}
	}
	return false
}

// DecryptMultiValueData decrypts all encrypted values in a data map
func DecryptMultiValueData(data map[string]interface{}, client *vault.Client, transitMount, keyName string) (map[string]interface{}, error) {
	decryptedData := make(map[string]interface{})
	
	for k, v := range data {
		if ciphertext, ok := v.(string); ok && strings.HasPrefix(ciphertext, "vault:v") {
			plaintext, err := client.TransitDecrypt(transitMount, keyName, ciphertext)
			if err != nil {
				return nil, fmt.Errorf("decrypt %s: %w", k, err)
			}
			decryptedData[k] = string(plaintext)
		} else {
			decryptedData[k] = v
		}
	}
	
	return decryptedData, nil
}

// OutputJSON outputs data as formatted JSON
func OutputJSON(data map[string]interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	fmt.Println(string(jsonData))
	return nil
}

// OutputEnvFormat outputs data in .env format
func OutputEnvFormat(data map[string]interface{}) {
	for k, v := range data {
		fmt.Printf("%s=%v\n", k, v)
	}
}

// MergeData merges new data into existing data, preserving existing values and adding/updating new ones
func MergeData(existing, new map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Copy existing data
	for k, v := range existing {
		result[k] = v
	}
	
	// Add/update with new data
	for k, v := range new {
		result[k] = v
	}
	
	return result
}