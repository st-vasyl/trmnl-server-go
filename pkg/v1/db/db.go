package db

import (
	"database/sql"
	"log"

	"github.com/thanhpk/randstr"
	_ "modernc.org/sqlite"
)

func InitDB(dbname string) error {
	db, err := sql.Open("sqlite", dbname)
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
		return err
	}
	log.Println("Table devices created successfully")
	return nil
}

func RegisterDevice(dbname, deviceId, apiKey, screen string) error {
	if apiKey == "" {
		apiKey = randstr.String(16)
	}
	db, err := sql.Open("sqlite", dbname)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer db.Close()

	_, err = db.Exec("INSERT INTO devices(device_id, api_key, screen, voltage) VALUES(?,?,?,0)", deviceId, apiKey, screen)
	if err != nil {
		log.Fatal(err)
		return err
	}
	log.Printf("New device %s with key %s registered successfully \n", deviceId, apiKey)
	return nil
}

func UpdateDevice(dbname, deviceId, voltage, screen string) error {
	db, err := sql.Open("sqlite", dbname)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer db.Close()

	_, err = db.Exec("UPDATE devices SET screen = ?, voltage = ? WHERE device_id = ?", screen, voltage, deviceId)
	if err != nil {
		log.Fatal(err)
		return err
	}
	log.Printf("DB: record for device %s updated successfully \n", deviceId)
	return nil
}

func GetDeviceScreen(dbname, deviceId string) (string, error) {
	var screen string
	db, err := sql.Open("sqlite", dbname)
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	defer db.Close()

	stmt, err := db.Prepare("SELECT screen FROM devices WHERE device_id = ?")
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	defer stmt.Close()

	err = stmt.QueryRow(deviceId).Scan(&screen)
	if err != nil {
		log.Printf("DB: Failed to get screen for device ID %s, error: %s", deviceId, err)
		return "", err
	}
	log.Printf("DB: found screen %s by device key %s \n", screen, deviceId)

	return screen, nil
}

func GetDeviceVoltage(dbname, apiKey string) (float32, error) {
	var voltage float32
	db, err := sql.Open("sqlite", dbname)
	if err != nil {
		log.Fatal(err)
		return 0, err
	}
	defer db.Close()

	stmt, err := db.Prepare("SELECT voltage FROM devices WHERE api_key = ?")
	if err != nil {
		log.Fatal(err)
		return 0, err
	}
	defer stmt.Close()

	err = stmt.QueryRow(apiKey).Scan(&voltage)
	if err != nil {
		log.Printf("DB: Failed to get voltage for api_key %s, error: %s", apiKey, err)
		return 0, err
	}
	log.Printf("DB: found voltage %.2f by device key %s \n", voltage, apiKey)

	return voltage, nil
}

func GetDeviceList(dbname string) ([]string, error) {
	var keys []string
	db, err := sql.Open("sqlite", dbname)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	defer db.Close()

	stmt, err := db.Prepare("SELECT api_key FROM devices")
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query()

	for rows.Next() {
		key := ""
		err := rows.Scan(&key)
		if err != nil {
			log.Fatal(err)
		}
		keys = append(keys, key)
	}
	log.Printf("DB: found device keys %s \n", keys)

	return keys, nil
}
