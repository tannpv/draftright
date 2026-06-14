package exttoken

import (
	"errors"
	"regexp"
)

var (
	// Node validates device_id with class-validator's @IsUUID('4') — strictly
	// a version-4 UUID (13th nibble == 4, 17th nibble in [89abAB]).
	uuid4Re = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	// Node: @Matches(/^[A-Za-z0-9 _.\-]+$/) on device_name.
	deviceNameRe = regexp.MustCompile(`^[A-Za-z0-9 _.\-]+$`)
)

// ValidateMint checks the mint inputs, returning an error with a stable
// message on the first failure, or nil. (T14 maps it to 400.)
//
// Node delegates device_id/length checks to class-validator (@IsUUID('4'),
// @Length(1, 64)) whose default messages are noisy; only the device_name
// charset message is author-supplied. We keep that one verbatim and use clear
// canonical messages for the other two.
func ValidateMint(deviceID, deviceName string) error {
	if !uuid4Re.MatchString(deviceID) {
		return errors.New("device_id must be a valid UUID")
	}
	if l := len(deviceName); l < 1 || l > 64 {
		return errors.New("device_name must be 1-64 characters")
	}
	if !deviceNameRe.MatchString(deviceName) {
		// Verbatim from Node DTO @Matches message.
		return errors.New("device_name must be alphanumeric/space/_/./- only")
	}
	return nil
}
