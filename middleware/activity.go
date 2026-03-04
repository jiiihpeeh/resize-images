package middleware

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
)

func NewActivityLogger(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()

		userID, ok := c.Locals("user_id").(string)
		if ok {
			go db.Exec("INSERT INTO activity_logs (user_id, action, details, ip_address) VALUES (?, ?, ?, ?)", userID, "resize", c.OriginalURL(), c.IP())
		}

		return err
	}
}
