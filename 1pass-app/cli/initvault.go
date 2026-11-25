// Creation of new, empty OPVault vaults.

package cli

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	clicontrol "github.com/mashmb/1pass/1pass-term/control/cobra"
	"golang.org/x/crypto/pbkdf2"
)

const vaultIterations = 100000

// InitVault creates an empty OPVault at path, protected by a master password.
func InitVault(path string) error {
	profileDir := filepath.Join(path, "default")

	if _, err := os.Stat(filepath.Join(profileDir, "profile.js")); err == nil {
		return fmt.Errorf("%v already contains a vault", path)
	}

	password, err := clicontrol.ReadPassword()

	if err != nil {
		return err
	}

	if len(password) == 0 {
		return errors.New("master password must not be empty")
	}

	salt := randomBytes(16)
	derived := pbkdf2.Key(password, salt, vaultIterations, 64, sha512.New)
	derivedKey, derivedMac := derived[:32], derived[32:]

	masterKey, err := encodeOpdata(randomBytes(256), derivedKey, derivedMac)

	if err != nil {
		return err
	}

	overviewKey, err := encodeOpdata(randomBytes(256), derivedKey, derivedMac)

	if err != nil {
		return err
	}

	now := time.Now().Unix()
	profile := map[string]interface{}{
		"lastUpdatedBy": "1pass",
		"updatedAt":     now,
		"profileName":   "default",
		"salt":          base64.StdEncoding.EncodeToString(salt),
		"passwordHint":  "",
		"masterKey":     base64.StdEncoding.EncodeToString(masterKey),
		"iterations":    vaultIterations,
		"uuid":          strings.ToUpper(hex.EncodeToString(randomBytes(16))),
		"overviewKey":   base64.StdEncoding.EncodeToString(overviewKey),
		"createdAt":     now,
	}

	body, err := json.Marshal(profile)

	if err != nil {
		return err
	}

	if err := os.MkdirAll(profileDir, 0700); err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(profileDir, "profile.js"), []byte("var profile="+string(body)+";"), 0600); err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(profileDir, "folders.js"), []byte("loadFolders({});"), 0600)
}

func randomBytes(n int) []byte {
	b := make([]byte, n)

	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	return b
}

// encodeOpdata produces an opdata01 blob (see the OPVault design document).
func encodeOpdata(plain, key, macKey []byte) ([]byte, error) {
	paddingSize := aes.BlockSize - (len(plain) % aes.BlockSize)
	padded := append(randomBytes(paddingSize), plain...)
	iv := randomBytes(aes.BlockSize)
	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	cipherText := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(cipherText, padded)

	var data bytes.Buffer
	data.WriteString("opdata01")
	size := make([]byte, 8)
	binary.LittleEndian.PutUint64(size, uint64(len(plain)))
	data.Write(size)
	data.Write(iv)
	data.Write(cipherText)

	mac := hmac.New(sha256.New, macKey)
	mac.Write(data.Bytes())

	return append(data.Bytes(), mac.Sum(nil)...), nil
}
