package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// Handler siklus hidup aset: perolehan, mutasi antar outlet, penghapusan,
// dan dashboard modul.

// ── Dashboard ───────────────────────────────────────────────

func AssetDashboard(c *fiber.Ctx) error {
	d, err := services.AssetDashboardStats(c.Query("outlet_id"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat dashboard aset: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: d})
}

// ── Perawatan lintas aset ───────────────────────────────────

func ListAllAssetMaintenances(c *fiber.Ctx) error {
	list, err := services.ListAllAssetMaintenances(
		c.Query("outlet_id"), c.Query("asset_id"), c.Query("type"),
		c.Query("from"), c.Query("to"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat riwayat perawatan: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

// ── Perolehan ───────────────────────────────────────────────

func ListAssetAcquisitions(c *fiber.Ctx) error {
	list, err := services.ListAssetAcquisitions(
		c.Query("outlet_id"), c.Query("asset_id"), c.Query("source"),
		c.Query("from"), c.Query("to"), c.Query("search"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat histori perolehan: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func AddAssetAcquisition(c *fiber.Ctx) error {
	var req models.AssetAcquisitionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	x, err := services.AddAssetAcquisition(req, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: x})
}

func DeleteAssetAcquisition(c *fiber.Ctx) error {
	if err := services.DeleteAssetAcquisition(c.Params("id"), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true})
}

// ── Mutasi ──────────────────────────────────────────────────

func ListAssetTransfers(c *fiber.Ctx) error {
	list, err := services.ListAssetTransfers(
		c.Query("outlet_id"), c.Query("asset_id"), c.Query("from"), c.Query("to"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat riwayat mutasi: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func TransferAsset(c *fiber.Ctx) error {
	var req models.AssetTransferRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	// Outlet tujuan harus ikut berada dalam akses pemakai — tanpa ini aset bisa
	// "dibuang" ke outlet yang tak terlihat oleh pemiliknya sendiri.
	if !validateOutletAccess(c, req.ToOutletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet tujuan di luar akses Anda"})
	}
	x, err := services.TransferAsset(req, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: x})
}

// ── Penghapusan ─────────────────────────────────────────────

func ListAssetDisposals(c *fiber.Ctx) error {
	list, err := services.ListAssetDisposals(
		c.Query("outlet_id"), c.Query("method"), c.Query("from"), c.Query("to"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat riwayat penghapusan: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func DisposeAsset(c *fiber.Ctx) error {
	var req models.AssetDisposalRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	x, err := services.DisposeAsset(req, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: x})
}

func RestoreDisposedAsset(c *fiber.Ctx) error {
	if err := services.RestoreDisposedAsset(c.Params("id"), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true})
}
