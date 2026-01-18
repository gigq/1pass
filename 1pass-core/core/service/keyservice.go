// Default implementation of key service.
//
// @author TSS

package service

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
	"io"

	"github.com/mashmb/1pass/1pass-core/core/domain"
	"github.com/mashmb/1pass/1pass-core/port/out"
)

type dfltKeyService struct {
	cryptoUtils out.CrytpoUtils
	profileRepo out.ProfileRepo
}

func NewDfltKeyService(cryptoUtils out.CrytpoUtils, profileRepo out.ProfileRepo) *dfltKeyService {
	return &dfltKeyService{
		cryptoUtils: cryptoUtils,
		profileRepo: profileRepo,
	}
}

func (s *dfltKeyService) CheckHmac(msg, key, desiredHmac []byte) error {
	hash := hmac.New(sha256.New, key)
	size, err := hash.Write(msg)

	if err != nil {
		return err
	}

	if size != len(msg) {
		return io.ErrShortWrite
	}

	computed := hash.Sum(nil)

	if !hmac.Equal(computed, desiredHmac) {
		return domain.ErrInvalidHmac
	}

	return nil
}

func (s *dfltKeyService) DecodeData(key, initVector, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	mode := cipher.NewCBCDecrypter(block, initVector)
	mode.CryptBlocks(data, data)

	return data, nil
}

func (s *dfltKeyService) EncodeData(key, initVector, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	if len(data)%aes.BlockSize != 0 {
		return nil, domain.ErrInvalidPayload
	}

	encoded := make([]byte, len(data))
	copy(encoded, data)
	mode := cipher.NewCBCEncrypter(block, initVector)
	mode.CryptBlocks(encoded, encoded)

	return encoded, nil
}

func (s *dfltKeyService) DecodeKeys(key, derivedKey, derivedMac []byte) ([]byte, []byte, error) {
	base, err := s.DecodeOpdata(key, derivedKey, derivedMac)

	if err != nil {
		return nil, nil, err
	}

	hash := sha512.New()

	if _, err := hash.Write(base); err != nil {
		return nil, nil, err
	}

	keys := hash.Sum(nil)

	return keys[:32], keys[32:64], nil
}

func (s *dfltKeyService) DecodeOpdata(cipherText, key, macKey []byte) ([]byte, error) {
	data, mac := cipherText[:len(cipherText)-32], cipherText[len(cipherText)-32:]

	if err := s.CheckHmac(data, macKey, mac); err != nil {
		return nil, err
	}

	var plainSize int64
	reader := bytes.NewReader(data[8:16])

	if err := binary.Read(reader, binary.LittleEndian, &plainSize); err != nil {
		return nil, err
	}

	plain, err := s.DecodeData(key, data[16:32], data[32:])

	if err != nil {
		return nil, err
	}

	return plain[len(plain)-int(plainSize):], nil
}

func (s *dfltKeyService) EncodeOpdata(plain, key, macKey []byte) ([]byte, error) {
	paddingSize := aes.BlockSize - (len(plain) % aes.BlockSize)

	if paddingSize == 0 {
		paddingSize = aes.BlockSize
	}

	padding := make([]byte, paddingSize)

	if _, err := rand.Read(padding); err != nil {
		return nil, err
	}

	padded := append(padding, plain...)
	initVector := make([]byte, aes.BlockSize)

	if _, err := rand.Read(initVector); err != nil {
		return nil, err
	}

	cipherText, err := s.EncodeData(key, initVector, padded)

	if err != nil {
		return nil, err
	}

	header := []byte("opdata01")
	size := make([]byte, 8)
	binary.LittleEndian.PutUint64(size, uint64(len(plain)))
	data := append(header, size...)
	data = append(data, initVector...)
	data = append(data, cipherText...)

	hash := hmac.New(sha256.New, macKey)

	if _, err := hash.Write(data); err != nil {
		return nil, err
	}

	mac := hash.Sum(nil)
	data = append(data, mac...)

	return data, nil
}

func (s *dfltKeyService) DerivedKeys(password string) ([]byte, []byte, error) {
	iterations := s.profileRepo.GetIterations()
	salt, err := base64.StdEncoding.DecodeString(s.profileRepo.GetSalt())

	if err != nil {
		return nil, nil, err
	}

	keys := s.cryptoUtils.DeriveKey([]byte(password), salt, iterations, 64, sha512.New)

	return keys[:32], keys[32:], nil
}

func (s *dfltKeyService) EncryptItemKeys(itemKey, itemMac []byte, keys *domain.Keys) ([]byte, error) {
	if len(itemKey) != 32 || len(itemMac) != 32 {
		return nil, domain.ErrInvalidPayload
	}

	initVector := make([]byte, aes.BlockSize)

	if _, err := rand.Read(initVector); err != nil {
		return nil, err
	}

	plain := append(itemKey, itemMac...)
	encrypted, err := s.EncodeData(keys.MasterKey, initVector, plain)

	if err != nil {
		return nil, err
	}

	data := append(initVector, encrypted...)
	hash := hmac.New(sha256.New, keys.MasterMac)

	if _, err := hash.Write(data); err != nil {
		return nil, err
	}

	mac := hash.Sum(nil)
	data = append(data, mac...)

	return data, nil
}

func (s *dfltKeyService) ItemKeys(item *domain.RawItem, keys *domain.Keys) ([]byte, []byte) {
	itemKeys, _ := base64.StdEncoding.DecodeString(item.Keys)
	data := itemKeys[:len(itemKeys)-32]
	plain, _ := s.DecodeData(keys.MasterKey, data[0:16], data[16:])

	return plain[0:32], plain[32:64]
}

func (s *dfltKeyService) MasterKeys(derivedKey, derivedMac []byte) ([]byte, []byte, error) {
	encoded, err := base64.StdEncoding.DecodeString(s.profileRepo.GetMasterKey())

	if err != nil {
		return nil, nil, err
	}

	return s.DecodeKeys(encoded, derivedKey, derivedMac)
}

func (s *dfltKeyService) OverviewKeys(derivedKey, derivedMac []byte) ([]byte, []byte, error) {
	encoded, err := base64.StdEncoding.DecodeString(s.profileRepo.GetOverviewKey())

	if err != nil {
		return nil, nil, err
	}

	return s.DecodeKeys(encoded, derivedKey, derivedMac)
}
