package db

import (
	"database/sql"

	"github.com/rs/zerolog/log"
	"github.com/thanhpk/randstr"
	_ "modernc.org/sqlite"
)

// Store owns the single *sql.DB handle for the devices table. Open it once at
// startup and pass it to consumers (handler, worker, screens) so they share the
// underlying connection pool.
type Store struct {
	db *sql.DB
}

// Open creates (or attaches to) the SQLite file at dbname and ensures the
// devices schema exists.
func Open(dbname string) (*Store, error) {
	d, err := sql.Open("sqlite", dbname)
	if err != nil {
		log.Error().Str("func", "Open").Str("dbname", dbname).Err(err).Msg("DB: unable to open database file")
		return nil, err
	}
	s := &Store{db: d}
	if err := s.migrate(); err != nil {
		d.Close()
		return nil, err
	}
	log.Info().Str("func", "Open").Msg("DB: devices schema ready")
	return s, nil
}

// Close releases the underlying *sql.DB.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	const stmt = `
    CREATE TABLE IF NOT EXISTS devices (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        device_id TEXT,
        api_key TEXT,
        screen TEXT,
        voltage NUM
    );`
	if _, err := s.db.Exec(stmt); err != nil {
		log.Error().Str("func", "migrate").Err(err).Msg("DB: unable to create devices table")
		return err
	}
	return nil
}

// RegisterDevice inserts a new device row. When apiKey is empty a 16-char
// random key is generated and stored; the caller does not learn the generated
// value (matching the legacy contract).
func (s *Store) RegisterDevice(deviceId, apiKey, screen string) error {
	if apiKey == "" {
		apiKey = randstr.String(16)
	}
	if _, err := s.db.Exec("INSERT INTO devices(device_id, api_key, screen, voltage) VALUES(?,?,?,0)", deviceId, apiKey, screen); err != nil {
		log.Error().Str("func", "RegisterDevice").Err(err).Msg("DB: insert failed")
		return err
	}
	log.Info().Str("func", "RegisterDevice").Str("device", deviceId).Str("api-key", apiKey).Msg("DB: new device registered")
	return nil
}

func (s *Store) UpdateDevice(deviceId, voltage, screen string) error {
	if _, err := s.db.Exec("UPDATE devices SET screen = ?, voltage = ? WHERE device_id = ?", screen, voltage, deviceId); err != nil {
		log.Error().Str("func", "UpdateDevice").Err(err).Msg("DB: update failed")
		return err
	}
	log.Info().Str("func", "UpdateDevice").Str("device", deviceId).Str("voltage", voltage).Str("screen", screen).Msg("DB: device updated")
	return nil
}

func (s *Store) GetDeviceScreen(deviceId string) (string, error) {
	var screen string
	if err := s.db.QueryRow("SELECT screen FROM devices WHERE device_id = ?", deviceId).Scan(&screen); err != nil {
		log.Error().Str("func", "GetDeviceScreen").Str("device", deviceId).Err(err).Msg("DB: no screen for device")
		return "", err
	}
	log.Info().Str("func", "GetDeviceScreen").Str("device", deviceId).Str("screen", screen).Msg("DB: found screen for device")
	return screen, nil
}

func (s *Store) GetDeviceVoltage(apiKey string) (float32, error) {
	var voltage float32
	if err := s.db.QueryRow("SELECT voltage FROM devices WHERE api_key = ?", apiKey).Scan(&voltage); err != nil {
		log.Error().Str("func", "GetDeviceVoltage").Str("api-key", apiKey).Err(err).Msg("DB: no voltage for device")
		return 0, err
	}
	log.Info().Str("func", "GetDeviceVoltage").Str("api-key", apiKey).Float32("voltage", voltage).Msg("DB: found voltage for device")
	return voltage, nil
}

func (s *Store) GetDeviceList() ([]string, error) {
	rows, err := s.db.Query("SELECT api_key FROM devices")
	if err != nil {
		log.Error().Str("func", "GetDeviceList").Err(err).Msg("DB: query failed")
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			log.Error().Str("func", "GetDeviceList").Err(err).Msg("DB: scan row failed")
			continue
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		log.Error().Str("func", "GetDeviceList").Err(err).Msg("DB: row iteration error")
		return nil, err
	}
	log.Info().Str("func", "GetDeviceList").Strs("api-keys", keys).Msg("DB: found device api-keys")
	return keys, nil
}
