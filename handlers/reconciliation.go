package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func adminName(c *fiber.Ctx) string {
	if u, ok := c.Locals("admin_username").(string); ok && u != "" {
		return u
	}
	return "admin"
}

// GetShiftReconciliation — bandingkan tutup kasir vs cloud per shift.
func GetShiftReconciliation(c *fiber.Ctx) error {
	dateFrom, dateTo, err := getDateRange(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	outletID := c.Query("outlet_id", "")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	report, err := services.GetShiftReconciliation(dateFrom, dateTo, outletID, getOutletScope(c))
	if err != nil {
		log.Printf("GetShiftReconciliation error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat rekonsiliasi shift: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: report})
}

// ApplyShiftAdjustment — samakan pendapatan cloud dengan versi kasir (revertible).
func ApplyShiftAdjustment(c *fiber.Ctx) error {
	shiftID := c.Params("shiftId")
	if err := services.ApplyShiftAdjustment(shiftID, adminName(c), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "Penyesuaian diterapkan — pendapatan mengikuti versi kasir"})
}

// RevertShiftAdjustment — batalkan penyesuaian, nilai kembali ke data cloud asli.
func RevertShiftAdjustment(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "ID penyesuaian tidak valid"})
	}
	if err := services.RevertShiftAdjustment(id, adminName(c), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "Penyesuaian dibatalkan — nilai dikembalikan"})
}
