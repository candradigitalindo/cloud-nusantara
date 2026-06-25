package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"cloud-pos/models"

	"github.com/gofiber/fiber/v2"
)

const appFilesDir = "uploads/apps"

var (
	appFileNameRe   = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	appAllowedExt   = map[string]bool{".apk": true, ".aab": true, ".ipa": true, ".zip": true, ".exe": true, ".dmg": true}
	maxAppFileBytes = int64(200 * 1024 * 1024) // 200MB
)

func sanitizeAppFileName(name string) string {
	name = filepath.Base(name)                  // strip any path
	name = strings.ReplaceAll(name, " ", "-")   // no spaces in URL
	name = appFileNameRe.ReplaceAllString(name, "_")
	name = strings.Trim(name, ".-_")
	if name == "" {
		name = "file"
	}
	return name
}

// uniqueAppFileName returns a name that doesn't yet exist in dir, appending
// -1, -2, … before the extension on collision (so a new upload never overwrites
// an existing file — each gets a unique name and download URL).
func uniqueAppFileName(dir, name string) string {
	if _, err := os.Stat(filepath.Join(dir, name)); os.IsNotExist(err) {
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(filepath.Join(dir, cand)); os.IsNotExist(err) {
			return cand
		}
	}
}

type appFileInfo struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Size       int64  `json:"size"`
	UploadedAt string `json:"uploaded_at"`
}

// ListAppFiles lists distributable app files (APK, etc.) with their download URLs.
func ListAppFiles(c *fiber.Ctx) error {
	entries, err := os.ReadDir(appFilesDir)
	if err != nil {
		// directory not created yet → empty list
		return c.JSON(models.APIResponse{Success: true, Data: []appFileInfo{}})
	}
	files := make([]appFileInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, appFileInfo{
			Name:       e.Name(),
			URL:        "/uploads/apps/" + e.Name(),
			Size:       info.Size(),
			UploadedAt: info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].UploadedAt > files[j].UploadedAt })
	return c.JSON(models.APIResponse{Success: true, Data: files})
}

// UploadAppFile saves an uploaded app file and returns its download URL.
func UploadAppFile(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "File tidak ditemukan."})
	}
	if file.Size > maxAppFileBytes {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Ukuran file maksimal 200MB."})
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !appAllowedExt[ext] {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format tidak didukung. Gunakan APK, AAB, IPA, ZIP, EXE, atau DMG."})
	}

	if err := os.MkdirAll(appFilesDir, 0755); err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menyiapkan folder."})
	}

	name := uniqueAppFileName(appFilesDir, sanitizeAppFileName(file.Filename))
	savePath := filepath.Join(appFilesDir, name)
	if err := c.SaveFile(file, savePath); err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menyimpan file."})
	}

	return c.Status(201).JSON(models.APIResponse{
		Success: true,
		Data: appFileInfo{
			Name:       name,
			URL:        "/uploads/apps/" + name,
			Size:       file.Size,
			UploadedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		},
	})
}

// DownloadAppFile serves an app file with the correct MIME type and a download
// disposition, so an APK is saved as ".apk" (not ".zip", which the generic ZIP
// content-sniffer would otherwise produce). Public — the URL is shareable.
func DownloadAppFile(c *fiber.Ctx) error {
	name := sanitizeAppFileName(c.Params("name"))
	path := filepath.Join(appFilesDir, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return c.Status(404).SendString("File tidak ditemukan")
	}

	ctype := "application/octet-stream"
	if strings.ToLower(filepath.Ext(name)) == ".apk" {
		ctype = "application/vnd.android.package-archive"
	}

	f, err := os.Open(path)
	if err != nil {
		return c.Status(500).SendString("Gagal membuka file")
	}
	c.Set("Content-Type", ctype)
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	c.Set("Cache-Control", "public, max-age=300")
	return c.SendStream(f, int(info.Size())) // SendStream closes the file when done
}

// DeleteAppFile removes an app file by name.
func DeleteAppFile(c *fiber.Ctx) error {
	name := sanitizeAppFileName(c.Params("name"))
	path := filepath.Join(appFilesDir, name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return c.Status(404).JSON(models.APIResponse{Success: false, Error: "File tidak ditemukan."})
		}
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menghapus file."})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"deleted": name}})
}

