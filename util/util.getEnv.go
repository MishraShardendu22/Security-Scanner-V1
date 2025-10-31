package util

import "os"

func GetEnv(key, fallback string) string {

	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

// Q. Iska use case kya hai ?

// A. Maine ye function senior ke codebase me deekha to standardise how we get env var.
// and handle their edge cases in a seperate utility function rather than scattering
