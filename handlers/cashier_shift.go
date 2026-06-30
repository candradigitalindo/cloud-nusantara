package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func GetCashierShiftReport(c *fiber.Ctx) error {
	outletID := c.Query("outlet_id")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	report, err := services.GetCashierShiftReport(
		outletID, c.Query("status"), c.Query("date_from"), c.Query("date_to"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat laporan shift kasir: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: report})
}
