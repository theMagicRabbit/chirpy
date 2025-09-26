package auth

import (
	"runtime"

	"github.com/alexedwards/argon2id"
)

func HashPassword(password string) (string, error) {
	params := &argon2id.Params{
		Memory: 128 * 1024,
		Parallelism: uint8(runtime.NumCPU()),
		SaltLength: 16,
		KeyLength: 32,
	}
	if passwordHash, err := argon2id.CreateHash(password, params); err != nil {
		return "", err
	} else {
		return passwordHash, nil
	}
}

func CheckPasswordHash(password, hash string) (bool, error) {
	params := &argon2id.Params{
		Memory: 128 * 1024,
		Parallelism: uint8(runtime.NumCPU()),
		SaltLength: 16,
		KeyLength: 32,
	}
	if passwordHash, err := argon2id.CreateHash(password, params); err != nil {
		return false, err
	} else {
		return (hash == passwordHash), nil
	}
}

