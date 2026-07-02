package handlers

import (
	"bufio"
	"fmt"
	"time"

	"cloud-pos/config"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// Events — stream SSE untuk update realtime dashboard admin.
// EventSource di browser tidak bisa mengirim header Authorization, jadi token
// dikirim via query (?token=) dan divalidasi manual dengan logika yang sama
// seperti middleware AdminAuth (static token / JWT).
func Events(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Query("token")
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(models.APIResponse{Success: false, Error: "Unauthorized"})
		}
		if token != cfg.AdminToken {
			if _, err := services.ValidateJWT(token, cfg.JWTSecret); err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(models.APIResponse{Success: false, Error: "Unauthorized: token tidak valid"})
			}
		}

		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		// Matikan buffering nginx untuk response ini agar event terkirim seketika.
		c.Set("X-Accel-Buffering", "no")

		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			sub := services.EventsHub.Subscribe()
			defer services.EventsHub.Unsubscribe(sub)

			// Ping berkala menjaga koneksi tetap hidup melewati proxy_read_timeout nginx (60s).
			ticker := time.NewTicker(25 * time.Second)
			defer ticker.Stop()

			fmt.Fprintf(w, ": connected\n\n")
			if err := w.Flush(); err != nil {
				return
			}
			for {
				select {
				case msg, ok := <-sub:
					if !ok {
						return
					}
					fmt.Fprintf(w, "data: %s\n\n", msg)
					if err := w.Flush(); err != nil {
						return
					}
				case <-ticker.C:
					fmt.Fprintf(w, ": ping\n\n")
					if err := w.Flush(); err != nil {
						return
					}
				}
			}
		}))
		return nil
	}
}
