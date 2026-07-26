package handlers

import (
	"database/sql"

	"cloud-pos/config"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// ── Monitoring & kelola kamera (admin, scoped) ──────────────────────────────

// GetCameras — daftar kamera semua outlet dalam scope (opsional ?outlet_id=).
func GetCameras(c *fiber.Ctx) error {
	outletID := c.Query("outlet_id")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	cams, err := services.ListCameras(outletID, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal memuat kamera: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: cams})
}

// GetOutletCameras — daftar kamera satu outlet.
func GetOutletCameras(c *fiber.Ctx) error {
	outletID := c.Params("id")
	if !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	cams, err := services.ListCamerasByOutlet(outletID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal memuat kamera: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: cams})
}

// CreateCamera — tambah kamera pada outlet (cameras.create).
func CreateCamera(c *fiber.Ctx) error {
	outletID := c.Params("id")
	if !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	var req models.CreateCameraRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format permintaan tidak valid"})
	}
	cam, err := services.CreateCamera(outletID, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal menambah kamera: " + err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: cam})
}

// UpdateCamera — ubah konfigurasi kamera (cameras.update).
func UpdateCamera(c *fiber.Ctx) error {
	id := c.Params("camId")
	if !cameraInScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses kamera tidak diizinkan"})
	}
	var req models.UpdateCameraRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format permintaan tidak valid"})
	}
	cam, err := services.UpdateCamera(id, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal memperbarui kamera: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: cam, Message: "Kamera berhasil diperbarui"})
}

// ToggleCamera — aktif/nonaktif (cameras.update).
func ToggleCamera(c *fiber.Ctx) error {
	id := c.Params("camId")
	if !cameraInScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses kamera tidak diizinkan"})
	}
	cam, err := services.ToggleCamera(id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal mengubah status kamera"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: cam})
}

// DeleteCamera — hapus kamera (cameras.delete).
func DeleteCamera(c *fiber.Ctx) error {
	id := c.Params("camId")
	if !cameraInScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses kamera tidak diizinkan"})
	}
	if err := services.DeleteCamera(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal menghapus kamera"})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "Kamera berhasil dihapus"})
}

// ── Streaming (live + playback) ─────────────────────────────────────────────

// StartCameraStream — mulai live view. Mendaftarkan stream ke go2rtc, menerbitkan
// cookie stream token (untuk relay /cctv/), dan mengembalikan URL HLS/WebRTC.
func StartCameraStream(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("camId")
		if !cameraInScope(c, id) {
			return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses kamera tidak diizinkan"})
		}
		resp, err := services.StartLive(id)
		if err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(models.APIResponse{Success: false, Error: "Gagal memulai stream: " + err.Error()})
		}
		if err := setCCTVCookie(c); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal menyiapkan sesi stream"})
		}
		return c.JSON(models.APIResponse{Success: true, Data: resp})
	}
}

// StartCameraPlayback — mulai putar ulang rekaman DVR untuk rentang waktu (transient).
func StartCameraPlayback(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("camId")
		if !cameraInScope(c, id) {
			return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses kamera tidak diizinkan"})
		}
		var req models.PlaybackRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format permintaan tidak valid"})
		}
		resp, err := services.StartPlayback(id, req)
		if err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(models.APIResponse{Success: false, Error: "Gagal memulai putar ulang: " + err.Error()})
		}
		if err := setCCTVCookie(c); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal menyiapkan sesi stream"})
		}
		return c.JSON(models.APIResponse{Success: true, Data: resp})
	}
}

// StopCameraStream — hentikan & bersihkan stream playback transient (?name=).
func StopCameraStream(c *fiber.Ctx) error {
	services.StopStream(c.Query("name"))
	return c.JSON(models.APIResponse{Success: true})
}

// CCTVAuthz — subrequest nginx auth_request untuk melindungi relay /cctv/.
// Divalidasi via cookie stream token (browser mengirimnya otomatis pada tiap
// request media/segmen). Registrasi di luar grup AdminAuth karena request media
// tidak membawa header Authorization.
func CCTVAuthz(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tok := c.Cookies("cctv_token")
		if tok == "" || services.ValidateCCTVToken(tok) != nil {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.SendStatus(fiber.StatusOK)
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// cameraInScope memuat outlet pemilik kamera lalu memvalidasi scope admin.
func cameraInScope(c *fiber.Ctx, camID string) bool {
	outletID, err := services.GetCameraOutletID(camID)
	if err == sql.ErrNoRows {
		return true // biarkan service mengembalikan "tidak ditemukan"
	}
	if err != nil {
		return false
	}
	return validateOutletAccess(c, outletID)
}

// setCCTVCookie menyetel cookie stream token (Path=/cctv, HttpOnly) berumur pendek.
func setCCTVCookie(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(string)
	tok, err := services.IssueCCTVToken(adminID)
	if err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{
		Name:     "cctv_token",
		Value:    tok,
		Path:     "/cctv",
		HTTPOnly: true,
		SameSite: "Lax",
		MaxAge:   3600,
	})
	return nil
}
