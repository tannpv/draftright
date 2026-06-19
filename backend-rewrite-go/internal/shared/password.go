package shared

import "golang.org/x/crypto/bcrypt"

// bcryptCost matches the NestJS BCRYPT_ROUNDS constant (10). bcryptjs
// and x/crypto/bcrypt share the Modular Crypt Format ($2a$/$2b$), so
// hashes written by either verify under the other — no migration.
const bcryptCost = 10

// HashPassword returns a bcrypt MCF hash for plain.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	return string(b), err
}

// VerifyPassword reports whether plain matches the stored bcrypt hash.
// A mismatch returns (false, nil); only malformed hashes return an error.
func VerifyPassword(plain, hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	if err == nil {
		return true, nil
	}
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return false, nil
	}
	return false, err
}
