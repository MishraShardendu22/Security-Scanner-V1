package util

import "github.com/gofiber/fiber/v2"

func ResponseAPI(c *fiber.Ctx, status int, message string, data any, token string) error {

	response := map[string]any{

		"status":  status,
		"message": message,
		"data":    data,
	}
	if token != "" {
		response["token"] = token
	}

	return c.Status(status).JSON(response)
}

// Q. Iska use case kya hai ?

// A. Standardise kar raha hu mai API responses ko,
// basically har ek request ka response jo milega,
// sabka format same rahega rather than being total hocus pocus.
