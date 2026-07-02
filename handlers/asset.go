package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func ListAssets(c *fiber.Ctx) error {
	assets, err := services.ListAssets(
		c.Query("outlet_id"), c.Query("search"), c.Query("condition"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat aset: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: assets})
}

func GetAsset(c *fiber.Ctx) error {
	a, err := services.GetAsset(c.Params("id"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{
			Success: false, Error: "Aset tidak ditemukan",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: a})
}

func CreateAsset(c *fiber.Ctx) error {
	var req models.AssetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	if !validateOutletAccess(c, req.OutletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet di luar akses Anda"})
	}
	a, err := services.CreateAsset(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: a})
}

func UpdateAsset(c *fiber.Ctx) error {
	var req models.AssetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	a, err := services.UpdateAsset(c.Params("id"), req, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: a})
}

func DeleteAsset(c *fiber.Ctx) error {
	if err := services.DeleteAsset(c.Params("id"), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true})
}

func ListAssetMaintenances(c *fiber.Ctx) error {
	// Scope guard: ensure the asset is visible before exposing its history.
	if _, err := services.GetAsset(c.Params("id"), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Success: false, Error: "Aset tidak ditemukan"})
	}
	list, err := services.ListAssetMaintenances(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat histori: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func AddAssetMaintenance(c *fiber.Ctx) error {
	var req models.AssetMaintenanceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	m, err := services.AddAssetMaintenance(c.Params("id"), req, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: m})
}

func DeleteAssetMaintenance(c *fiber.Ctx) error {
	if err := services.DeleteAssetMaintenance(c.Params("id"), c.Params("mid"), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true})
}
