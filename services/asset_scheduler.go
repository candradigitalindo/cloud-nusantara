package services

import (
	"cloud-pos/database"
	"fmt"
	"log"
	"time"
)

// StartAssetScheduler mengevaluasi jadwal perawatan perlengkapan sekali sehari
// (sekitar pukul 02:00 zona waktu aplikasi) dan saat boot: menghitung aset yang
// perawatannya terlambat atau segera jatuh tempo, lalu broadcast SSE
// "asset_alert" supaya halaman Perlengkapan yang sedang terbuka menyegar sendiri
// begitu sebuah jadwal berpindah status di pergantian hari.
func StartAssetScheduler() {
	go func() {
		evaluateAssetMaintenanceAlerts()
		lastRunDay := ""
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			loc := GetTimezoneLocation()
			now := time.Now().In(loc)
			day := now.Format("2006-01-02")
			if now.Hour() == 2 && day != lastRunDay {
				lastRunDay = day
				evaluateAssetMaintenanceAlerts()
			}
		}
	}()
}

func evaluateAssetMaintenanceAlerts() {
	var overdue, dueSoon int
	err := database.DB.QueryRow(fmt.Sprintf(`
		SELECT COUNT(CASE WHEN nd.next_due_date < CURRENT_DATE THEN 1 END)::int,
		       COUNT(CASE WHEN nd.next_due_date >= CURRENT_DATE AND nd.next_due_date <= CURRENT_DATE + %d THEN 1 END)::int
		FROM assets a%s
		WHERE a.is_deleted = false`, assetDueSoonDays, assetNextDueJoin)).Scan(&overdue, &dueSoon)
	if err != nil {
		log.Printf("[Aset] Evaluasi jadwal perawatan gagal: %v", err)
		return
	}

	log.Printf("[Aset] Evaluasi jadwal perawatan: %d terlambat, %d jatuh tempo ≤%d hari", overdue, dueSoon, assetDueSoonDays)
	if overdue > 0 || dueSoon > 0 {
		BroadcastSync("asset_alert", "")
	}
}
