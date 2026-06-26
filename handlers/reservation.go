package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// ── Admin ───────────────────────────────────────────────────

func ListReservations(c *fiber.Ctx) error {
	data, err := services.ListReservations(
		c.Query("outlet_id"), c.Query("status"), c.Query("date_from"), c.Query("date_to"), getOutletScope(c))
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal memuat reservasi: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: data})
}

func GetReservation(c *fiber.Ctx) error {
	r, err := services.GetReservation(c.Params("id"), getOutletScope(c))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: "Reservasi tidak ditemukan"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: r})
}

func CreateReservation(c *fiber.Ctx) error {
	var req models.ReservationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	r, err := services.CreateReservation(req, getOutletScope(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: r})
}

func UpdateReservation(c *fiber.Ctx) error {
	var req models.ReservationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	r, err := services.UpdateReservation(c.Params("id"), req, getOutletScope(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: r})
}

func UpdateReservationStatus(c *fiber.Ctx) error {
	var body struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&body); err != nil || body.Status == "" {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Status tidak valid"})
	}
	r, err := services.UpdateReservationStatus(c.Params("id"), body.Status, getOutletScope(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: r})
}

func DeleteReservation(c *fiber.Ctx) error {
	if err := services.DeleteReservation(c.Params("id"), getOutletScope(c)); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true})
}

// ── Public (no auth) ────────────────────────────────────────

func PublicGetMenu(c *fiber.Ctx) error {
	menu, err := services.GetPublicMenu(c.Params("slug"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: "Outlet tidak ditemukan"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: menu})
}

func PublicCreateReservation(c *fiber.Ctx) error {
	var req models.ReservationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	r, err := services.CreatePublicReservation(c.Params("slug"), req)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: fiber.Map{
		"id": r.ID, "customer_name": r.CustomerName, "status": r.Status,
		"reservation_date": r.ReservationDate, "reservation_time": r.ReservationTime,
		"total": r.Total, "down_payment": r.DownPayment, "remaining": r.Remaining,
	}})
}
