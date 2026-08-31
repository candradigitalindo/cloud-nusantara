package services

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Alur tagihan QRIS, dari sisi kasir:
//
//  1. POS memanggil CreateQRISCharge saat kasir memilih metode QRIS. Baris
//     tagihan tersimpan berstatus pending dan QR-nya dikirim balik.
//  2. Tamu memindai lalu membayar. Penyedia memanggil webhook kita, yang
//     memanggil MarkQRISChargePaid.
//  3. POS mem-polling GetQRISCharge sampai statusnya paid, lalu barulah
//     mencatat pembayaran seperti biasa (tunai/kartu) ke shift berjalan.
//
// POS tetap yang mencatat pembayarannya sendiri — cloud hanya memastikan uang
// benar-benar masuk. Dengan begitu tagihan QRIS tidak pernah membuat pembayaran
// muncul di cloud tanpa pasangannya di laporan shift kasir.

// CreateQRISCharge membuat tagihan baru lewat penyedia aktif dan menyimpannya.
//
// Bila untuk order yang sama masih ada tagihan pending yang belum kedaluwarsa,
// tagihan ITU yang dikembalikan alih-alih membuat yang baru — kasir yang tidak
// sengaja menekan "QRIS" dua kali tidak menagih tamu dua kali.
func CreateQRISCharge(outletID, orderLocalID string, amount float64, description string) (*models.QRISChargeResponse, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("nominal tagihan harus lebih dari 0")
	}

	if existing, err := findReusableCharge(outletID, orderLocalID, amount); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	gw, err := ActiveGateway()
	if err != nil {
		return nil, err
	}

	chargeID := newChargeID()
	res, err := gw.CreateQRIS(QRISCharge{
		ChargeID:    chargeID,
		OutletID:    outletID,
		Amount:      amount,
		Description: description,
	})
	if err != nil {
		return nil, fmt.Errorf("penyedia menolak tagihan: %w", err)
	}

	_, err = database.DB.Exec(
		`INSERT INTO qris_charges (id, outlet_id, order_local_id, amount, provider,
			provider_ref, qr_string, status, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8, NOW(), NOW())`,
		chargeID, outletID, nullStr(orderLocalID), amount, gw.Name(),
		res.ProviderRef, res.QRString, res.ExpiresAt.UTC(),
	)
	if err != nil {
		return nil, err
	}

	go logSync(outletID, "qris_charge_created", "qris_charge", 1, "success", "")

	return &models.QRISChargeResponse{
		ChargeID:  chargeID,
		Provider:  gw.Name(),
		QRString:  res.QRString,
		Amount:    amount,
		Status:    QRISPending,
		ExpiresAt: res.ExpiresAt.UTC().Format(time.RFC3339),
	}, nil
}

// findReusableCharge mencari tagihan pending yang masih berlaku untuk order &
// nominal yang sama. Nominal ikut dicocokkan supaya perubahan tagihan (item
// ditambah setelah QR terbit) tetap menerbitkan QR baru.
func findReusableCharge(outletID, orderLocalID string, amount float64) (*models.QRISChargeResponse, error) {
	if orderLocalID == "" {
		return nil, nil
	}
	var r models.QRISChargeResponse
	var expires time.Time
	err := database.DB.QueryRow(
		`SELECT id, provider, qr_string, amount, status, expires_at
		FROM qris_charges
		WHERE outlet_id = $1 AND order_local_id = $2 AND status = 'pending'
			AND amount = $3 AND expires_at > NOW()
		ORDER BY created_at DESC LIMIT 1`,
		outletID, orderLocalID, amount,
	).Scan(&r.ChargeID, &r.Provider, &r.QRString, &r.Amount, &r.Status, &expires)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.ExpiresAt = expires.UTC().Format(time.RFC3339)
	return &r, nil
}

// GetQRISCharge mengembalikan status terkini sebuah tagihan — endpoint yang
// di-polling POS sambil menunggu tamu membayar.
//
// Tagihan pending yang sudah lewat waktu ditandai expired di sini, sehingga
// polling berhenti walau penyedia tidak pernah mengirim apa pun.
func GetQRISCharge(outletID, chargeID string) (*models.QRISChargeResponse, error) {
	var r models.QRISChargeResponse
	var expires time.Time
	var paidAt sql.NullTime
	err := database.DB.QueryRow(
		`SELECT id, provider, qr_string, amount, status, expires_at, paid_at
		FROM qris_charges WHERE outlet_id = $1 AND id = $2`,
		outletID, chargeID,
	).Scan(&r.ChargeID, &r.Provider, &r.QRString, &r.Amount, &r.Status, &expires, &paidAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tagihan tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	if r.Status == QRISPending && time.Now().UTC().After(expires.UTC()) {
		if _, uerr := database.DB.Exec(
			`UPDATE qris_charges SET status = 'expired', updated_at = NOW()
			WHERE id = $1 AND status = 'pending'`, chargeID,
		); uerr != nil {
			log.Printf("tandai qris_charge %s expired: %v", chargeID, uerr)
		} else {
			r.Status = QRISExpired
		}
	}

	r.ExpiresAt = expires.UTC().Format(time.RFC3339)
	if paidAt.Valid {
		r.PaidAt = paidAt.Time.UTC().Format(time.RFC3339)
	}
	return &r, nil
}

// MarkQRISChargePaid mencatat hasil dari webhook penyedia.
//
// Idempoten dan sekali-jalan: hanya baris yang MASIH pending yang berubah, jadi
// callback ganda (penyedia mengulang kiriman) tidak pernah menghasilkan
// pembayaran dobel, dan tagihan yang sudah lunas tidak bisa diturunkan lagi
// menjadi gagal oleh callback yang datang terlambat.
func MarkQRISChargePaid(chargeID, status string) error {
	if status != QRISPaid && status != QRISExpired && status != QRISFailed {
		return nil // pending — tidak ada yang perlu diubah
	}

	var paidAt interface{}
	if status == QRISPaid {
		paidAt = time.Now().UTC()
	}

	res, err := database.DB.Exec(
		`UPDATE qris_charges SET status = $1, paid_at = $2, updated_at = NOW()
		WHERE id = $3 AND status = 'pending'`,
		status, paidAt, chargeID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Bukan error: callback ulang atas tagihan yang sudah selesai.
		log.Printf("webhook qris %s diabaikan — tagihan tidak lagi pending", chargeID)
		return nil
	}

	if status == QRISPaid {
		// Tagihan ini mungkin milik pesanan online. Baru SETELAH lunas pesanan
		// itu boleh dilihat POS — dapur tidak pernah memasak pesanan yang belum
		// dibayar. Dijalankan hanya saat baris benar-benar berubah menjadi
		// lunas, jadi callback ganda tidak mempromosikannya dua kali.
		if err := promoteOnlineOrderAfterPayment(chargeID); err != nil {
			// Uang sudah masuk — jangan gagalkan webhook. Dicatat supaya
			// pesanan yang tertinggal bisa ditelusuri.
			log.Printf("promosi pesanan online untuk tagihan %s gagal: %v", chargeID, err)
		}
	}
	return nil
}

// promoteOnlineOrderAfterPayment memindahkan pesanan dari awaiting_payment ke
// new, mencatat nominal yang benar-benar dibayar.
func promoteOnlineOrderAfterPayment(chargeID string) error {
	var amount float64
	if err := database.DB.QueryRow(
		`SELECT amount FROM qris_charges WHERE id = $1`, chargeID,
	).Scan(&amount); err != nil {
		return err
	}
	_, err := database.DB.Exec(
		`UPDATE online_orders
		SET status = 'new', paid_amount = $2, updated_at = NOW()
		WHERE qris_charge_id = $1 AND status = 'awaiting_payment'`,
		chargeID, amount,
	)
	return err
}
