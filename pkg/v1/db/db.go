package db

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB(dbname string) error {
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer db.Close()
	sqlStmt := `
    CREATE TABLE IF NOT EXISTS devices (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
				key TEXT,
				screen TEXT
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

func RegisterDevice(dbname, key, screen string) error {
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer db.Close()

	_, err = db.Exec("INSERT INTO devices(key, screen) VALUES(?,?)", key, screen)
	if err != nil {
		log.Fatal(err)
		return err
	}
	log.Printf("New device %s registered successfully \n", key)
	return nil
}

func UpdateDevice(dbname, key, screen string) error {
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer db.Close()

	_, err = db.Exec("UPDATE devices SET screen = ? WHERE key = ?", screen, key)
	if err != nil {
		log.Fatal(err)
		return err
	}
	log.Printf("Device %s updated successfully \n", key)
	return nil
}

func GetDevice(dbname, key string) (string, error) {
	var screen string
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	defer db.Close()

	stmt, err := db.Prepare("SELECT screen FROM devices WHERE key = ?")
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	defer stmt.Close()

	err = stmt.QueryRow(key).Scan(&screen)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Found screen %s by device api-key %s \n", screen, key)

	return screen, nil
}
