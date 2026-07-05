package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

// tzExpr — sub-ekspresi SQL untuk timezone aktif (fallback Asia/Jakarta).
const tzExpr = "COALESCE((SELECT value FROM app_settings WHERE key='timezone'),'Asia/Jakarta')"

// Ambang status perangkat (menit sejak heartbeat terakhir diterima cloud).
const (
	deviceOnlineMaxMin = 15 // <= online
	deviceIdleMaxMin   = 60 // <= idle, selebihnya offline
)

// SaveDeviceHeartbeat menyimpan snapshot terakhir perangkat (upsert per outlet)
// dan satu baris histori untuk tren. Toleran: heartbeat basi diabaikan app, jadi
// kegagalan di sini tidak boleh mengganggu sync data.
func SaveDeviceHeartbeat(outletID string, req models.DeviceHeartbeatRequest) error {
	if outletID == "" {
		return fmt.Errorf("outlet_id kosong")
	}

	if req.Printers == nil {
		req.Printers = []models.DevicePrinter{}
	}
	printersJSON, err := json.Marshal(req.Printers)
	if err != nil {
		printersJSON = []byte("[]")
	}
	pTotal, pOnline := 0, 0
	for _, p := range req.Printers {
		pTotal++
		if p.Connected || p.Online {
			pOnline++
		}
	}

	reportedAt := parseTime(req.ReportedAt)
	lastSyncAt := parseTime(req.Network.LastSyncAt)

	d := req.Device
	_, err = database.DB.Exec(`
		INSERT INTO device_heartbeats
			(outlet_id, app_version, battery, battery_state, model, os,
			 storage_total_mb, storage_free_mb, network_online, pending_sync,
			 last_sync_at, printers, reported_at,
			 ram_total_mb, ram_free_mb, ram_used_percent, ram_low,
			 cpu_cores, cpu_used_percent, cpu_load_1m, cpu_load_5m, cpu_load_15m,
			 received_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,
			$14,$15,$16,$17,$18,$19,$20,$21,$22, now() AT TIME ZONE 'UTC')
		ON CONFLICT (outlet_id) DO UPDATE SET
			app_version=$2, battery=$3, battery_state=$4, model=$5, os=$6,
			storage_total_mb=$7, storage_free_mb=$8, network_online=$9, pending_sync=$10,
			last_sync_at=$11, printers=$12, reported_at=$13,
			ram_total_mb=$14, ram_free_mb=$15, ram_used_percent=$16, ram_low=$17,
			cpu_cores=$18, cpu_used_percent=$19, cpu_load_1m=$20, cpu_load_5m=$21, cpu_load_15m=$22,
			received_at=now() AT TIME ZONE 'UTC'`,
		outletID, d.AppVersion, d.Battery, d.BatteryState,
		d.Model, d.OS, d.StorageTotalMB, d.StorageFreeMB,
		req.Network.Online, req.Network.PendingSync, lastSyncAt, string(printersJSON), reportedAt,
		d.RamTotalMB, d.RamFreeMB, d.RamUsedPercent, d.RamLow,
		d.CPUCores, d.CPUUsedPercent, d.CPULoad1m, d.CPULoad5m, d.CPULoad15m,
	)
	if err != nil {
		return err
	}

	// Histori ringkas untuk tren (best-effort).
	database.DB.Exec(`
		INSERT INTO device_heartbeat_logs
			(outlet_id, battery, battery_state, network_online, pending_sync,
			 printers_online, printers_total, reported_at, received_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now() AT TIME ZONE 'UTC')`,
		outletID, req.Device.Battery, req.Device.BatteryState, req.Network.Online,
		req.Network.PendingSync, pOnline, pTotal, reportedAt,
	)
	// Pangkas histori > 7 hari agar tidak menumpuk.
	database.DB.Exec(`DELETE FROM device_heartbeat_logs
		WHERE outlet_id = $1 AND received_at < (now() AT TIME ZONE 'UTC') - interval '7 days'`, outletID)

	return nil
}

// GetDeviceMonitor mengembalikan kondisi perangkat tiap outlet aktif (dalam scope),
// termasuk outlet yang belum pernah mengirim heartbeat (status no_data).
func GetDeviceMonitor(outletID string, outletScope []string) (*models.DeviceMonitorReport, error) {
	conds := []string{"o.is_active = true"}
	args := []interface{}{}
	idx := 1
	if outletID != "" {
		conds = append(conds, fmt.Sprintf("o.id = $%d", idx))
		args = append(args, outletID)
		idx++
	}
	if outletScope != nil {
		conds = append(conds, fmt.Sprintf("o.id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(outletScope))
		idx++
	}

	q := fmt.Sprintf(`
		SELECT o.id, o.name, COALESCE(o.code,''),
			(h.outlet_id IS NOT NULL),
			COALESCE(h.app_version,''), COALESCE(h.battery,-1), COALESCE(h.battery_state,''),
			COALESCE(h.model,''), COALESCE(h.os,''),
			COALESCE(h.storage_total_mb,0), COALESCE(h.storage_free_mb,0),
			COALESCE(h.network_online,false), COALESCE(h.pending_sync,0),
			COALESCE(to_char((h.last_sync_at AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI'),''),
			COALESCE(to_char((h.reported_at  AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI'),''),
			COALESCE(to_char((h.received_at  AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI'),''),
			COALESCE(EXTRACT(EPOCH FROM ((now() AT TIME ZONE 'UTC') - h.received_at))/60, -1)::int,
			COALESCE(h.printers::text, '[]'),
			h.ram_total_mb, h.ram_free_mb, h.ram_used_percent, h.ram_low,
			h.cpu_cores, h.cpu_used_percent, h.cpu_load_1m, h.cpu_load_5m, h.cpu_load_15m
		FROM outlets o
		LEFT JOIN device_heartbeats h ON h.outlet_id = o.id
		WHERE %[2]s
		ORDER BY o.name ASC`, tzExpr, strings.Join(conds, " AND "))

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	report := &models.DeviceMonitorReport{Devices: []models.DeviceStatus{}}
	for rows.Next() {
		var d models.DeviceStatus
		var printersJSON string
		var ramTotal, ramFree, ramUsed, cpuCores sql.NullInt64
		var ramLow sql.NullBool
		var cpuUsed, cpuL1, cpuL5, cpuL15 sql.NullFloat64
		if err := rows.Scan(&d.OutletID, &d.OutletName, &d.OutletCode,
			&d.HasData, &d.AppVersion, &d.Battery, &d.BatteryState, &d.Model, &d.OS,
			&d.StorageTotalMB, &d.StorageFreeMB, &d.NetworkOnline, &d.PendingSync,
			&d.LastSyncAt, &d.ReportedAt, &d.ReceivedAt, &d.StaleMinutes, &printersJSON,
			&ramTotal, &ramFree, &ramUsed, &ramLow,
			&cpuCores, &cpuUsed, &cpuL1, &cpuL5, &cpuL15); err != nil {
			return nil, err
		}
		d.OutletID = strings.TrimSpace(d.OutletID)
		// Kolom nullable → pointer (null bila perangkat tidak melaporkannya).
		if ramTotal.Valid {
			v := int(ramTotal.Int64)
			d.RamTotalMB = &v
		}
		if ramFree.Valid {
			v := int(ramFree.Int64)
			d.RamFreeMB = &v
		}
		if ramUsed.Valid {
			v := int(ramUsed.Int64)
			d.RamUsedPercent = &v
		}
		if ramLow.Valid {
			d.RamLow = &ramLow.Bool
		}
		if cpuCores.Valid {
			v := int(cpuCores.Int64)
			d.CPUCores = &v
		}
		if cpuUsed.Valid {
			d.CPUUsedPercent = &cpuUsed.Float64
		}
		if cpuL1.Valid {
			d.CPULoad1m = &cpuL1.Float64
		}
		if cpuL5.Valid {
			d.CPULoad5m = &cpuL5.Float64
		}
		if cpuL15.Valid {
			d.CPULoad15m = &cpuL15.Float64
		}

		var printers []models.DevicePrinter
		json.Unmarshal([]byte(printersJSON), &printers)
		if printers == nil {
			printers = []models.DevicePrinter{}
		}
		d.Printers = printers
		for _, p := range printers {
			d.PrintersTotal++
			if p.Connected || p.Online {
				d.PrintersOnline++
			}
		}

		// Status berdasarkan keterlambatan heartbeat.
		switch {
		case !d.HasData:
			d.Status = "no_data"
		case d.StaleMinutes <= deviceOnlineMaxMin:
			d.Status = "online"
		case d.StaleMinutes <= deviceIdleMaxMin:
			d.Status = "idle"
		default:
			d.Status = "offline"
		}

		if d.StorageTotalMB > 0 {
			d.StorageUsedPct = round2(float64(d.StorageTotalMB-d.StorageFreeMB) / float64(d.StorageTotalMB) * 100)
		}

		// Ringkasan armada.
		report.Summary.TotalOutlets++
		report.Summary.PendingSyncTotal += d.PendingSync
		switch d.Status {
		case "online":
			report.Summary.OnlineCount++
		case "idle":
			report.Summary.IdleCount++
		case "offline":
			report.Summary.OfflineCount++
		case "no_data":
			report.Summary.NoDataCount++
		}
		if d.HasData && d.Battery >= 0 && d.Battery < 20 &&
			d.BatteryState != "charging" && d.BatteryState != "full" {
			report.Summary.LowBattery++
		}
		if d.HasData && d.PrintersOnline < d.PrintersTotal {
			report.Summary.PrinterIssues++
		}
		// Perangkat dengan sumber daya kritis: RAM ≥90% / lowMemory, atau CPU ≥90%.
		if d.HasData {
			ramCrit := (d.RamUsedPercent != nil && *d.RamUsedPercent >= 90) || (d.RamLow != nil && *d.RamLow)
			cpuCrit := d.CPUUsedPercent != nil && *d.CPUUsedPercent >= 90
			if ramCrit || cpuCrit {
				report.Summary.ResourceIssues++
			}
		}

		report.Devices = append(report.Devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Urutkan: yang bermasalah dulu (offline → no_data → idle → online), lalu nama.
	prio := map[string]int{"offline": 0, "no_data": 1, "idle": 2, "online": 3}
	sort.SliceStable(report.Devices, func(i, j int) bool {
		pi, pj := prio[report.Devices[i].Status], prio[report.Devices[j].Status]
		if pi != pj {
			return pi < pj
		}
		return report.Devices[i].OutletName < report.Devices[j].OutletName
	})

	return report, nil
}

// GetDeviceHistory mengembalikan histori heartbeat terakhir (maks 48 titik) sebuah outlet.
func GetDeviceHistory(outletID string) ([]models.DeviceHeartbeatPoint, error) {
	q := fmt.Sprintf(`
		SELECT battery, COALESCE(battery_state,''), network_online, pending_sync,
			printers_online, printers_total,
			COALESCE(to_char((reported_at AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'MM-DD HH24:MI'),'')
		FROM (
			SELECT * FROM device_heartbeat_logs
			WHERE outlet_id = $1
			ORDER BY received_at DESC
			LIMIT 48
		) t
		ORDER BY received_at ASC`, tzExpr)

	rows, err := database.DB.Query(q, outletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.DeviceHeartbeatPoint, 0)
	for rows.Next() {
		var p models.DeviceHeartbeatPoint
		if err := rows.Scan(&p.Battery, &p.BatteryState, &p.NetworkOnline, &p.PendingSync,
			&p.PrintersOnline, &p.PrintersTotal, &p.ReportedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
