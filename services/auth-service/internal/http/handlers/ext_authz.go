package handlers

import "github.com/gofiber/fiber/v2"

func ExtAuthzOK(c *fiber.Ctx) error {
	mode := "public"
	if token, ok := c.Locals("api_key").(string); ok && token != "" {
		mode = "token"
	}
	c.Set("X-Auth-Mode", mode)
	return c.SendStatus(fiber.StatusOK)
}
