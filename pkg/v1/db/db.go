package db

import (
	"database/sql"

	"github.com/rs/zerolog/log"

	"github.com/thanhpk/randstr"
	_ "modernc.org/sqlite"
)

func InitDB(dbname string) error {
	db, err := sql.Open("sqlite", dbname)
	if err != nil {
		log.Error().Str("func", "InitDB").Str("dbname", dbname).Err(err).Msg("DB: Unable to open database file")
		return err
	}
	defer db.Close()
	sqlStmt := `
    CREATE TABLE IF NOT EXISTS devices (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
				device_id TEXT,
				api_key TEXT,
				screen TEXT,
				voltage NUM
    );
    `
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Error().Str("func", "InitDB").Str("dbname", dbname).Err(err).Msg("DB: Unable to exec SQL statement")
		return err
	}
	log.Info().Str("func", "InitDB").Msg("DB: Table devices created successfully")
	return nil
}

func RegisterDevice(dbname, deviceId, apiKey, screen string) error {
	if apiKey == "" {
		apiKey = randstr.String(16)
	}
	db, err := sql.Open("sqlite", dbname)
	if err != nil {
		log.Error().Str("func", "RegisterDevice").Str("dbname", dbname).Err(err).Msg("DB: Unable to open database file")
		return err
	}
	defer db.Close()

	_, err = db.Exec("INSERT INTO devices(device_id, api_key, screen, voltage) VALUES(?,?,?,0)", deviceId, apiKey, screen)
	if err != nil {
		log.Error().Str("func", "RegisterDevice").Str("dbname", dbname).Err(err).Msg("DB: Unable to exec SQL statement")
		return err
	}
	log.Info().Str("func", "RegisterDevice").Str("device", deviceId).Str("api-key", apiKey).
		Msg("DB: New device registered successfully")
	return nil
}

func UpdateDevice(dbname, deviceId, voltage, screen string) error {
	db, err := sql.Open("sqlite", dbname)
	if err != nil {
		log.Error().Str("func", "UpdateDevice").Str("dbname", dbname).Err(err).Msg("DB: Unable to open database file")
		return err
	}
	defer db.Close()

	_, err = db.Exec("UPDATE devices SET screen = ?, voltage = ? WHERE device_id = ?", screen, voltage, deviceId)
	if err != nil {
		log.Error().Str("func", "UpdateDevice").Str("dbname", dbname).Err(err).Msg("DB: Unable to exec SQL statement")
		return err
	}
	log.Info().Str("func", "UpdateDevice").Str("device", deviceId).Str("voltage", voltage).Str("screen", screen).
		Msg("DB: device updated successfully")
	return nil
}

func GetDeviceScreen(dbname, deviceId string) (string, error) {
	var screen string
	db, err := sql.Open("sqlite", dbname)
	if err != nil {
		log.Error().Str("func", "GetDeviceScreen").Str("dbname", dbname).Err(err).Msg("DB: Unable to open database file")
		return "", err
	}
	defer db.Close()

	stmt, err := db.Prepare("SELECT screen FROM devices WHERE device_id = ?")
	if err != nil {
		log.Error().Str("func", "GetDeviceScreen").Str("dbname", dbname).Err(err).Msg("DB: Unable to exec SQL statement")
		return "", err
	}
	defer stmt.Close()

	err = stmt.QueryRow(deviceId).Scan(&screen)
	if err != nil {
		log.Error().Str("func", "GetDeviceScreen").Str("dbname", dbname).Str("device", deviceId).Err(err).Msg("DB: Unable to find a screen for the device")

		return "", err
	}
	log.Info().Str("func", "GetDeviceScreen").Str("device", deviceId).Str("screen", screen).Msg("DB: Found screen for the device")

	return screen, nil
}

func GetDeviceVoltage(dbname, apiKey string) (float32, error) {
	var voltage float32
	db, err := sql.Open("sqlite", dbname)
	if err != nil {
		log.Error().Str("func", "GetDeviceVoltage").Str("dbname", dbname).Err(err).Msg("DB: Unable to open database file")
		return 0, err
	}
	defer db.Close()

	stmt, err := db.Prepare("SELECT voltage FROM devices WHERE api_key = ?")
	if err != nil {
		log.Error().Str("func", "GetDeviceVoltage").Str("dbname", dbname).Err(err).Msg("DB: Unable to exec SQL statement")
		return 0, err
	}
	defer stmt.Close()

	err = stmt.QueryRow(apiKey).Scan(&voltage)
	if err != nil {
		log.Error().Str("func", "GetDeviceVoltage").Str("dbname", dbname).Str("api-key", apiKey).Float32("voltage", voltage).Err(err).
			Msg("DB: Unable to find a voltage for the device")
		return 0, err
	}
	log.Info().Str("func", "GetDeviceVoltage").Str("dbname", dbname).Str("api-key", apiKey).Float32("voltage", voltage).
		Msg("DB: Found voltagee for the device")

	return voltage, nil
}

func GetDeviceList(dbname string) ([]string, error) {
	var keys []string
	db, err := sql.Open("sqlite", dbname)
	if err != nil {
		log.Error().Str("func", "GetDeviceList").Str("dbname", dbname).Err(err).Msg("DB: Unable to open database file")
		return nil, err
	}
	defer db.Close()

	stmt, err := db.Prepare("SELECT api_key FROM devices")
	if err != nil {
		log.Error().Str("func", "GetDeviceList").Str("dbname", dbname).Err(err).Msg("DB: Unable to exec SQL statement")
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query()

	for rows.Next() {
		key := ""
		err := rows.Scan(&key)
		if err != nil {
			log.Error().Str("func", "GetDeviceList").Str("dbname", dbname).Err(err).Msg("DB: Unable to Scan rows in SQL responce")
		}
		keys = append(keys, key)
	}

	log.Info().Str("func", "GetDeviceList").Str("dbname", dbname).Strs("api-keys", keys).Msg("DB: Found device api-keys")

	return keys, nil
}
