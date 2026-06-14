package exttoken

// Exported test surface so the external exttoken_test package can exercise the
// unexported token primitives.

var (
	GenerateToken = generateToken
	HashToken     = hashToken
)
