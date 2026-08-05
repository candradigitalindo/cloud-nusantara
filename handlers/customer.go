package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func ListCustomers(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	data, total, err := services.ListCustomers(c.Query("search"), getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal memuat pelanggan: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{
		"customers": data,
		"pagination": fiber.Map{"page": page, "limit": limit, "total": total},
	}})
}

func GetCustomer(c *fiber.Ctx) error {
	d, err := services.GetCustomerDetail(c.Params("id"), getOutletScope(c))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: d})
}
