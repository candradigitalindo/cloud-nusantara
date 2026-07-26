package services

// CCTV — integrasi kamera CCTV outlet secara RELAY-ONLY (zero-retention).
//
// Prinsip:
//   - Cloud TIDAK PERNAH menyimpan video. go2rtc hanya me-relay/transcode aliran
//     RTSP dari DVR outlet ke browser (WebRTC/HLS in-memory). Rekaman tetap di DVR.
//   - Cloud tidak bisa masuk ke LAN outlet (NAT). go2rtc menjangkau DVR lewat overlay
//     privat (Tailscale/WireGuard) yang di-dial keluar dari edge outlet.
//   - Kredensial RTSP disimpan terenkripsi (AES-256-GCM) dan hanya dipakai server-side;
//     dikirim ke go2rtc via API internal (tidak pernah ke browser).
//
// Alur: handler membangun URL RTSP (live/playback) → daftarkan stream ke go2rtc via
// API internal → browser menonton lewat nginx /cctv/ (dilindungi cookie stream token).

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lib/pq"
)

var (
	cctvEncKey    []byte // 32-byte AES key (turunan SHA-256)
	cctvSecret    string // secret untuk stream token (HS256)
	go2rtcBaseURL string
	cctvHTTP      = &http.Client{Timeout: 6 * time.Second}
)

// InitCCTV menyiapkan kunci enkripsi, secret token, dan alamat go2rtc. Dipanggil
// sekali dari main setelah config di-load. encKeyRaw boleh kosong — kunci diturunkan
// dari jwtSecret agar tetap berfungsi tanpa env tambahan (bisa dioverride via CAMERA_ENC_KEY).
func InitCCTV(encKeyRaw, jwtSecret, go2rtcURL string) {
	seed := encKeyRaw
	if seed == "" {
		seed = "cctv:" + jwtSecret
	}
	sum := sha256.Sum256([]byte(seed))
	cctvEncKey = sum[:]
	cctvSecret = jwtSecret
	go2rtcBaseURL = strings.TrimRight(go2rtcURL, "/")
	if go2rtcBaseURL == "" {
		go2rtcBaseURL = "http://go2rtc:1984"
	}
}

// ── Enkripsi kredensial RTSP (AES-256-GCM) ──────────────────────────────────

func encryptSecret(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	block, err := aes.NewCipher(cctvEncKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

func decryptSecret(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(cctvEncKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("ciphertext terlalu pendek")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// ── Stream token (cookie berumur pendek untuk akses relay /cctv/) ───────────

// IssueCCTVToken menerbitkan JWT bertipe "cctv" (HS256) untuk admin tertentu,
// dipakai sebagai cookie yang divalidasi nginx auth_request pada tiap request /cctv/.
func IssueCCTVToken(adminID string) (string, error) {
	claims := jwt.MapClaims{
		"typ": "cctv",
		"sub": adminID,
		"iat": time.Now().UTC().Unix(),
		"exp": time.Now().UTC().Add(1 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cctvSecret))
}

// ValidateCCTVToken memvalidasi cookie stream token.
func ValidateCCTVToken(tok string) error {
	parsed, err := jwt.Parse(tok, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("metode tanda tangan tidak sesuai")
		}
		return []byte(cctvSecret), nil
	})
	if err != nil {
		return err
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return fmt.Errorf("token tidak valid")
	}
	if t, _ := claims["typ"].(string); t != "cctv" {
		return fmt.Errorf("tipe token bukan cctv")
	}
	return nil
}

// ── Pembangun URL RTSP (Hikvision / Dahua) ──────────────────────────────────

func normBrand(b string) string {
	b = strings.ToLower(strings.TrimSpace(b))
	if strings.HasPrefix(b, "dahua") {
		return "dahua"
	}
	return "hikvision"
}

func camPort(p int) int {
	if p <= 0 {
		return 554
	}
	return p
}

func rtspBase(cam models.OutletCamera, pass string) *url.URL {
	u := &url.URL{Scheme: "rtsp", Host: fmt.Sprintf("%s:%d", cam.Host, camPort(cam.Port))}
	if cam.RTSPUsername != "" {
		u.User = url.UserPassword(cam.RTSPUsername, pass)
	}
	return u
}

// BuildLiveRTSP menyusun URL RTSP live sub-stream (default) sesuai merk.
//   Hikvision: /Streaming/Channels/<ch><stream>  (ch1 sub = 102, main = 101)
//   Dahua:     /cam/realmonitor?channel=<ch>&subtype=<0|1>
func BuildLiveRTSP(cam models.OutletCamera, pass string) string {
	u := rtspBase(cam, pass)
	ch := cam.Channel
	if ch <= 0 {
		ch = 1
	}
	if normBrand(cam.Brand) == "dahua" {
		st := 1
		if cam.Subtype == 0 {
			st = 0
		}
		u.Path = "/cam/realmonitor"
		u.RawQuery = fmt.Sprintf("channel=%d&subtype=%d", ch, st)
		return u.String()
	}
	// Hikvision
	streamNo := 2 // sub-stream
	if cam.Subtype == 0 {
		streamNo = 1 // main-stream
	}
	u.Path = fmt.Sprintf("/Streaming/Channels/%d", ch*100+streamNo)
	return u.String()
}

// BuildPlaybackRTSP menyusun URL RTSP putar-ulang dari rekaman DVR untuk rentang waktu.
// Aliran playback bersifat transient — cloud tidak menyimpan apa pun.
//   Hikvision: /Streaming/tracks/<ch>01?starttime=YYYYMMDDThhmmssZ&endtime=...
//   Dahua:     /cam/playback?channel=<ch>&subtype=0&starttime=YYYY_MM_DD_hh_mm_ss&endtime=...
func BuildPlaybackRTSP(cam models.OutletCamera, pass, start, end string) (string, error) {
	st, err := parsePlaybackTime(start)
	if err != nil {
		return "", fmt.Errorf("waktu mulai tidak valid")
	}
	et, err := parsePlaybackTime(end)
	if err != nil {
		return "", fmt.Errorf("waktu selesai tidak valid")
	}
	if !et.After(st) {
		return "", fmt.Errorf("waktu selesai harus setelah waktu mulai")
	}
	u := rtspBase(cam, pass)
	ch := cam.Channel
	if ch <= 0 {
		ch = 1
	}
	if normBrand(cam.Brand) == "dahua" {
		u.Path = "/cam/playback"
		u.RawQuery = fmt.Sprintf("channel=%d&subtype=0&starttime=%s&endtime=%s",
			ch, st.Format("2006_01_02_15_04_05"), et.Format("2006_01_02_15_04_05"))
		return u.String(), nil
	}
	// Hikvision — starttime/endtime dalam format ISO basic + 'Z'.
	u.Path = fmt.Sprintf("/Streaming/tracks/%d", ch*100+1)
	u.RawQuery = fmt.Sprintf("starttime=%s&endtime=%s",
		st.Format("20060102T150405Z"), et.Format("20060102T150405Z"))
	return u.String(), nil
}

func parsePlaybackTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, f := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("format waktu tidak dikenal")
}

// ── Klien go2rtc (API internal, di jaringan compose — bukan ke outlet) ───────

// Go2rtcPublish mendaftarkan/memperbarui satu named-stream di go2rtc (idempotent).
// go2rtc menghubungkan RTSP secara on-demand (hanya saat ada penonton) dan tidak
// menulis apa pun ke disk (relay in-memory).
func Go2rtcPublish(name, src string) error {
	q := url.Values{"name": {name}, "src": {src}}
	req, err := http.NewRequest(http.MethodPut, go2rtcBaseURL+"/api/streams?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := cctvHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("go2rtc tidak dapat dihubungi: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("go2rtc gagal daftar stream (%d): %s", resp.StatusCode, bytes.TrimSpace(body))
	}
	return nil
}

// Go2rtcDelete menghapus named-stream (dipakai untuk membersihkan aliran playback transient).
func Go2rtcDelete(name string) error {
	q := url.Values{"src": {name}}
	req, err := http.NewRequest(http.MethodDelete, go2rtcBaseURL+"/api/streams?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := cctvHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// ── CRUD kamera (config saja) ───────────────────────────────────────────────

func scanCameras(rows *sql.Rows) ([]models.OutletCamera, error) {
	defer rows.Close()
	out := make([]models.OutletCamera, 0)
	for rows.Next() {
		var c models.OutletCamera
		var pwdEnc, outletName string
		if err := rows.Scan(&c.ID, &c.OutletID, &outletName, &c.Name, &c.Brand, &c.Host, &c.Port,
			&c.Channel, &c.Subtype, &c.RTSPUsername, &pwdEnc, &c.IsActive, &c.SortOrder,
			&c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.ID = strings.TrimSpace(c.ID)
		c.OutletID = strings.TrimSpace(c.OutletID)
		c.OutletName = strings.TrimSpace(outletName)
		c.HasPassword = pwdEnc != ""
		out = append(out, c)
	}
	return out, rows.Err()
}

const cameraCols = `c.id, c.outlet_id, COALESCE(o.name,''), c.name, c.brand, c.host, c.port,
	c.channel, c.subtype, c.rtsp_username, c.rtsp_password_enc, c.is_active, c.sort_order,
	c.created_at, c.updated_at`

// ListCameras mengembalikan kamera semua outlet dalam scope (opsional filter satu outlet).
func ListCameras(outletID string, scope []string) ([]models.OutletCamera, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	if outletID != "" {
		conds = append(conds, fmt.Sprintf("c.outlet_id = $%d", idx))
		args = append(args, strings.TrimSpace(outletID))
		idx++
	}
	if scope != nil {
		conds = append(conds, fmt.Sprintf("c.outlet_id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(scope))
		idx++
	}
	q := fmt.Sprintf(`SELECT %s FROM outlet_cameras c
		LEFT JOIN outlets o ON o.id = c.outlet_id
		WHERE %s ORDER BY o.name ASC, c.sort_order ASC, c.created_at ASC`,
		cameraCols, strings.Join(conds, " AND "))
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	return scanCameras(rows)
}

// ListCamerasByOutlet mengembalikan kamera satu outlet.
func ListCamerasByOutlet(outletID string) ([]models.OutletCamera, error) {
	q := fmt.Sprintf(`SELECT %s FROM outlet_cameras c
		LEFT JOIN outlets o ON o.id = c.outlet_id
		WHERE c.outlet_id = $1 ORDER BY c.sort_order ASC, c.created_at ASC`, cameraCols)
	rows, err := database.DB.Query(q, strings.TrimSpace(outletID))
	if err != nil {
		return nil, err
	}
	return scanCameras(rows)
}

// GetCamera mengembalikan satu kamera (tanpa password).
func GetCamera(id string) (*models.OutletCamera, error) {
	q := fmt.Sprintf(`SELECT %s FROM outlet_cameras c
		LEFT JOIN outlets o ON o.id = c.outlet_id WHERE c.id = $1`, cameraCols)
	rows, err := database.DB.Query(q, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	list, err := scanCameras(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, sql.ErrNoRows
	}
	return &list[0], nil
}

// GetCameraOutletID mengembalikan outlet_id pemilik kamera (untuk validasi scope).
func GetCameraOutletID(id string) (string, error) {
	var outletID string
	err := database.DB.QueryRow(`SELECT outlet_id FROM outlet_cameras WHERE id = $1`, strings.TrimSpace(id)).Scan(&outletID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(outletID), nil
}

// getCameraSecret mengembalikan kamera + password RTSP terdekripsi (server-side only).
func getCameraSecret(id string) (*models.OutletCamera, string, error) {
	cam, err := GetCamera(id)
	if err != nil {
		return nil, "", err
	}
	var enc string
	if err := database.DB.QueryRow(`SELECT rtsp_password_enc FROM outlet_cameras WHERE id = $1`, strings.TrimSpace(id)).Scan(&enc); err != nil {
		return nil, "", err
	}
	pass, err := decryptSecret(enc)
	if err != nil {
		return nil, "", fmt.Errorf("gagal dekripsi kredensial kamera")
	}
	return cam, pass, nil
}

func CreateCamera(outletID string, req models.CreateCameraRequest) (*models.OutletCamera, error) {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Host) == "" {
		return nil, fmt.Errorf("nama dan host wajib diisi")
	}
	encPwd, err := encryptSecret(req.RTSPPassword)
	if err != nil {
		return nil, err
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	ch := req.Channel
	if ch <= 0 {
		ch = 1
	}
	id := NewULID()
	_, err = database.DB.Exec(`INSERT INTO outlet_cameras
		(id, outlet_id, name, brand, host, port, channel, subtype, rtsp_username, rtsp_password_enc, is_active, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		id, strings.TrimSpace(outletID), strings.TrimSpace(req.Name), normBrand(req.Brand),
		strings.TrimSpace(req.Host), camPort(req.Port), ch, req.Subtype,
		strings.TrimSpace(req.RTSPUsername), encPwd, active, req.SortOrder)
	if err != nil {
		return nil, err
	}
	return GetCamera(id)
}

func UpdateCamera(id string, req models.UpdateCameraRequest) (*models.OutletCamera, error) {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Host) == "" {
		return nil, fmt.Errorf("nama dan host wajib diisi")
	}
	ch := req.Channel
	if ch <= 0 {
		ch = 1
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	// Password: hanya ditimpa bila dikirim (non-nil). String kosong = kosongkan.
	if req.RTSPPassword != nil {
		encPwd, err := encryptSecret(*req.RTSPPassword)
		if err != nil {
			return nil, err
		}
		_, err = database.DB.Exec(`UPDATE outlet_cameras SET
			name=$1, brand=$2, host=$3, port=$4, channel=$5, subtype=$6, rtsp_username=$7,
			rtsp_password_enc=$8, is_active=$9, sort_order=$10, updated_at=(now() AT TIME ZONE 'UTC')
			WHERE id=$11`,
			strings.TrimSpace(req.Name), normBrand(req.Brand), strings.TrimSpace(req.Host),
			camPort(req.Port), ch, req.Subtype, strings.TrimSpace(req.RTSPUsername),
			encPwd, active, req.SortOrder, strings.TrimSpace(id))
		if err != nil {
			return nil, err
		}
	} else {
		_, err := database.DB.Exec(`UPDATE outlet_cameras SET
			name=$1, brand=$2, host=$3, port=$4, channel=$5, subtype=$6, rtsp_username=$7,
			is_active=$8, sort_order=$9, updated_at=(now() AT TIME ZONE 'UTC')
			WHERE id=$10`,
			strings.TrimSpace(req.Name), normBrand(req.Brand), strings.TrimSpace(req.Host),
			camPort(req.Port), ch, req.Subtype, strings.TrimSpace(req.RTSPUsername),
			active, req.SortOrder, strings.TrimSpace(id))
		if err != nil {
			return nil, err
		}
	}
	return GetCamera(id)
}

func ToggleCamera(id string) (*models.OutletCamera, error) {
	res, err := database.DB.Exec(`UPDATE outlet_cameras SET is_active = NOT is_active, updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$1`, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, sql.ErrNoRows
	}
	return GetCamera(id)
}

func DeleteCamera(id string) error {
	res, err := database.DB.Exec(`DELETE FROM outlet_cameras WHERE id=$1`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// StartLive membangun stream live di go2rtc dan mengembalikan info stream.
func StartLive(id string) (*models.StreamResponse, error) {
	cam, pass, err := getCameraSecret(id)
	if err != nil {
		return nil, err
	}
	if !cam.IsActive {
		return nil, fmt.Errorf("kamera nonaktif")
	}
	name := "cam_" + cam.ID
	if err := Go2rtcPublish(name, BuildLiveRTSP(*cam, pass)); err != nil {
		return nil, err
	}
	return streamResp(name, "live"), nil
}

// StartPlayback membangun stream putar-ulang transient di go2rtc.
func StartPlayback(id string, req models.PlaybackRequest) (*models.StreamResponse, error) {
	cam, pass, err := getCameraSecret(id)
	if err != nil {
		return nil, err
	}
	src, err := BuildPlaybackRTSP(*cam, pass, req.Start, req.End)
	if err != nil {
		return nil, err
	}
	// Nama unik agar sesi playback tidak saling menimpa; dibersihkan saat Stop.
	nonce := NewULID()
	name := "pb_" + cam.ID + "_" + strings.ToLower(nonce[len(nonce)-8:])
	if err := Go2rtcPublish(name, src); err != nil {
		return nil, err
	}
	return streamResp(name, "playback"), nil
}

// StopStream menghapus named-stream dari go2rtc (best-effort).
func StopStream(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	_ = Go2rtcDelete(name)
}

func streamResp(name, mode string) *models.StreamResponse {
	return &models.StreamResponse{
		Name:      name,
		Mode:      mode,
		HLSURL:    "/cctv/api/stream.m3u8?src=" + url.QueryEscape(name),
		WebRTCURL: "/cctv/api/webrtc?src=" + url.QueryEscape(name),
	}
}
