// Package user owns the customer identity record. Phase 1a exposes
// reads + password update + account deletion; register/verification
// land in Phase 1b.
package user

import "time"

// User is the domain projection used by auth. PasswordHash is empty for
// social-only accounts (no local password).
type User struct {
	ID                   string
	Email                string
	PasswordHash         string
	Name                 string
	IsActive             bool
	Role                 string
	AuthProvider         string // local | google | facebook | tiktok
	EmailVerified        bool
	LemonsqueezyCustomer string // "" when none
	AvatarURL            string // "" when null
}

// NewUser is the insert payload. Optional string fields are "" when
// absent; SocialProvider/SocialID set exactly one *_id column (""
// provider = local signup, no social column).
type NewUser struct {
	Email                    string
	PasswordHash             string // "" → NULL (social accounts)
	Name                     string
	AuthProvider             string // "local" default at the repo
	AvatarURL                string
	EmailVerified            bool
	EmailVerificationCode    string // "" → NULL
	EmailVerificationExpires *time.Time
	SocialProvider           string // "google"|"facebook"|"tiktok"|"apple"|""
	SocialID                 string
}

// UserPatch is a partial update — nil pointer / unset = column untouched.
type UserPatch struct {
	PasswordHash             *string
	EmailVerified            *bool
	EmailVerificationCode    NullableString // set+null vs untouched
	EmailVerificationExpires NullableTime
	PasswordResetCode        NullableString
	PasswordResetExpires     NullableTime
	PasswordResetAttempts    *int
	AvatarURL                *string
	SocialProvider           string // for social link: which *_id col
	SocialID                 string
}

// NullableString distinguishes "leave column alone" (Set=false) from
// "set column to NULL" (Set=true, Value=nil) from "set value".
type NullableString struct {
	Set   bool
	Value *string
}

// NullableTime mirrors NullableString for timestamptz columns.
type NullableTime struct {
	Set   bool
	Value *time.Time
}

// AuthState is the verification/reset projection the lifecycle endpoints
// read (separate from the login projection so Phase 1a is untouched).
type AuthState struct {
	ID                       string
	Email                    string
	Name                     string
	PasswordHash             string
	AuthProvider             string
	IsActive                 bool
	EmailVerified            bool
	EmailVerificationCode    string
	EmailVerificationExpires *time.Time
	PasswordResetCode        string
	PasswordResetExpires     *time.Time
	PasswordResetAttempts    int
}
