# Kontrak Waktu (Timestamp) — App POS → Cloud

## Masalah

Timestamp yang dikirim app **tanpa penanda zona waktu** (mis.
`"2026-06-29T16:02:00"`) **dianggap UTC** oleh server. Karena waktu sebenarnya
adalah **WIB (UTC+7)**, laporan cloud menambah +7 jam lagi saat menampilkan →
**transaksi sore/malam tergeser +7 jam dan melompat ke hari berikutnya**.

Contoh nyata: transaksi 29 Jun pukul 16:02–21:55 WIB tercatat di cloud sebagai
**30 Jun** dini hari. Omzet hari itu jadi salah tanggal.

## Aturan (WAJIB)

> **Semua timestamp di payload harus ISO 8601 dengan zona waktu eksplisit.**

Pilih salah satu (konsisten):

| Cara | Contoh | Keterangan |
|---|---|---|
| **UTC + `Z`** (disarankan) | `2026-06-29T09:02:00Z` | Kirim nilai dalam UTC, akhiri `Z`. |
| **Offset lokal** | `2026-06-29T16:02:00+07:00` | Waktu lokal + offset `+07:00`. |

**JANGAN** kirim waktu lokal polos tanpa zona (`2026-06-29T16:02:00`) — ini yang
menyebabkan pergeseran.

> Kedua bentuk di atas mewakili **momen yang sama**; server menyimpannya sebagai
> UTC dan mengonversi ke zona outlet (Asia/Jakarta) saat menampilkan. Tidak ada
> perubahan di sisi server yang diperlukan bila app sudah mengirim zona.

## Field yang terdampak

Berlaku untuk **semua** timestamp di payload `order` dan `transaction`:

- `transaction_date`  ← paling penting (jadi tanggal di laporan)
- `created_at`, `updated_at`
- `payments[].created_at`
- `payment_info.voided_at`, `payment_info.paid_at`
- (dan timestamp lain bila ada)

## Sebelum / Sesudah

```jsonc
// ❌ SALAH (dianggap UTC → geser +7 jam)
{ "transaction_date": "2026-06-29T16:02:00", "created_at": "2026-06-29T16:02:00" }

// ✅ BENAR — opsi A (UTC + Z)
{ "transaction_date": "2026-06-29T09:02:00Z", "created_at": "2026-06-29T09:02:00Z" }

// ✅ BENAR — opsi B (offset lokal)
{ "transaction_date": "2026-06-29T16:02:00+07:00", "created_at": "2026-06-29T16:02:00+07:00" }
```

## Implementasi Flutter (saran)

Gunakan `DateTime.toUtc().toIso8601String()` (menghasilkan akhiran `Z`), atau
sertakan offset. Hindari `DateTime.toString()` / format manual tanpa zona.

```dart
final ts = DateTime.now().toUtc().toIso8601String(); // "2026-06-29T09:02:00.000Z"
```

## Sumber payload

`app-pos-flutter/lib/repositories/order_repository.dart` →
`_enqueueTransaction`, `splitBillPayment` (dan pembuatan order) — pastikan setiap
field timestamp memakai bentuk ber-zona di atas.
