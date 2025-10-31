package util

import (
	"github.com/google/uuid"
)

func GenerateRequestID() string {
	return uuid.New().String()
}

func ValidateOrgParam(org string) error {
	if org == "" {
		return ErrOrgRequired
	}
	return nil
}

func ValidateResourceIDParam(id string) error {
	if id == "" {
		return ErrResourceIDRequired
	}
	return nil
}

// Q. What is the use case of this file?
// A. Bass fucntions jo boht often use karna hai unko likha h.
// Jaise ki UUID generate karna hia
// org ka controller ho and org he naa de to ? isliye ValidateOrgParam
// same for resource id.
