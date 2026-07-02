package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// PushDeviceHeartbeat menerima telemetri perangkat dari App POS (auth via API key
// outlet). Sengaja toleran: selalu balas 200 agar kegagalan tak mengganggu sync app.
func PushDeviceHeartbeat(c *fiber.Ctx) error {
	outletID, _ := c.Locals("outlet_id").(string)
	outletID = strings.TrimSpace(outletID)

	var req models.DeviceHeartbeatRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Format heartbeat tidak valid",
		})
	}

	if err := services.SaveDeviceHeartbeat(outletID, req); err != nil {
		log.Printf("SaveDeviceHeartbeat error (outlet %s): %v", outletID, err)
		// Tetap balas sukses-lunak; app tidak melakukan retry heartbeat.
		return c.JSON(models.APIResponse{Success: false, Error: "Gagal menyimpan heartbeat"})
	}
	services.BroadcastSync("device", outletID)
	return c.JSON(models.APIResponse{Success: true})
}

// GetDeviceMonitor — daftar kondisi perangkat per outlet (admin, scoped).
func GetDeviceMonitor(c *fiber.Ctx) error {
	outletID := c.Query("outlet_id")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	report, err := services.GetDeviceMonitor(outletID, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat status perangkat: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: report})
}

// GetDeviceHistory — histori heartbeat satu outlet (admin, scoped).
func GetDeviceHistory(c *fiber.Ctx) error {
	outletID := strings.TrimSpace(c.Params("outletId"))
	if !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	points, err := services.GetDeviceHistory(outletID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat histori perangkat: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: points})
}
