package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var db *sql.DB

type Soal struct {
	ID               int64    `json:"id"`
	Grade            string   `json:"grade"`
	Subject          string   `json:"subject"`
	Semester         int      `json:"semester"`
	ChapterNum       int      `json:"chapter_num"`
	ChapterTitle     string   `json:"chapter_title"`
	Question         string   `json:"question"`
	Options          []string `json:"options"`
	CorrectAnswerIdx int      `json:"correct_answer_idx"`
	TimeLimit        int      `json:"time_limit"`
}

func initDB() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	var err error
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Gagal konek ke database: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("Database tidak merespon: %v", err)
	}

	log.Println("Berhasil terhubung ke Supabase PostgreSQL!")
}

func getSitePasscodeFromDB() string {
	var passcode string
	err := db.QueryRow("SELECT value FROM settings WHERE key = 'site_passcode'").Scan(&passcode)
	if err != nil {
		return "BELAJAR2026"
	}
	return passcode
}

func updateSitePasscodeInDB(newPasscode string) error {
	_, err := db.Exec("INSERT INTO settings (key, value) VALUES ('site_passcode', $1) ON CONFLICT (key) DO UPDATE SET value = $1, updated_at = NOW()", newPasscode)
	return err
}

func saveSoalBatch(soalList []Soal) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO soal (grade, subject, semester, chapter_num, chapter_title, question, options, correct_answer_idx, time_limit) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, s := range soalList {
		optsJSON, err := json.Marshal(s.Options)
		if err != nil {
			return err
		}
		_, err = stmt.Exec(s.Grade, s.Subject, s.Semester, s.ChapterNum, s.ChapterTitle, s.Question, optsJSON, s.CorrectAnswerIdx, s.TimeLimit)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func getAllSoalDB() ([]Soal, error) {
	rows, err := db.Query(`SELECT id, grade, subject, semester, chapter_num, chapter_title, question, options, correct_answer_idx, time_limit FROM soal ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Soal
	for rows.Next() {
		var s Soal
		var optsJSON []byte
		err := rows.Scan(&s.ID, &s.Grade, &s.Subject, &s.Semester, &s.ChapterNum, &s.ChapterTitle, &s.Question, &optsJSON, &s.CorrectAnswerIdx, &s.TimeLimit)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(optsJSON, &s.Options)
		result = append(result, s)
	}
	return result, nil
}

func deleteSoalDB(id int64) error {
	_, err := db.Exec(`DELETE FROM soal WHERE id = $1`, id)
	return err
}

func updateSoalDB(s Soal) error {
	optsJSON, err := json.Marshal(s.Options)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE soal SET grade=$1, subject=$2, semester=$3, chapter_num=$4, chapter_title=$5, question=$6, options=$7, correct_answer_idx=$8, time_limit=$9 WHERE id=$10`,
		s.Grade, s.Subject, s.Semester, s.ChapterNum, s.ChapterTitle, s.Question, optsJSON, s.CorrectAnswerIdx, s.TimeLimit, s.ID)
	return err
}

func insertSingleSoalDB(s Soal) error {
	optsJSON, err := json.Marshal(s.Options)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO soal (grade, subject, semester, chapter_num, chapter_title, question, options, correct_answer_idx, time_limit) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		s.Grade, s.Subject, s.Semester, s.ChapterNum, s.ChapterTitle, s.Question, optsJSON, s.CorrectAnswerIdx, s.TimeLimit)
	return err
}

func fetchSoalFiltered(grade, subject, mode string, semester, chapter int) ([]Soal, error) {
	var query string
	var args []interface{}

	switch mode {
	case "chapter":
		query = `SELECT id, grade, subject, semester, chapter_num, chapter_title, question, options, correct_answer_idx, time_limit 
                 FROM soal WHERE grade=$1 AND subject=$2 AND semester=$3 AND chapter_num=$4 ORDER BY RANDOM()`
		args = []interface{}{grade, subject, semester, chapter}
	case "sem1":
		query = `SELECT id, grade, subject, semester, chapter_num, chapter_title, question, options, correct_answer_idx, time_limit 
                 FROM soal WHERE grade=$1 AND subject=$2 AND semester=1 ORDER BY RANDOM()`
		args = []interface{}{grade, subject}
	case "sem2":
		query = `SELECT id, grade, subject, semester, chapter_num, chapter_title, question, options, correct_answer_idx, time_limit 
                 FROM soal WHERE grade=$1 AND subject=$2 AND semester=2 ORDER BY RANDOM()`
		args = []interface{}{grade, subject}
	case "full":
		query = `SELECT id, grade, subject, semester, chapter_num, chapter_title, question, options, correct_answer_idx, time_limit 
                 FROM soal WHERE grade=$1 AND subject=$2 ORDER BY RANDOM()`
		args = []interface{}{grade, subject}
	default:
		return nil, fmt.Errorf("mode filter tidak valid")
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Soal
	for rows.Next() {
		var s Soal
		var optsJSON []byte
		err := rows.Scan(&s.ID, &s.Grade, &s.Subject, &s.Semester, &s.ChapterNum, &s.ChapterTitle, &s.Question, &optsJSON, &s.CorrectAnswerIdx, &s.TimeLimit)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(optsJSON, &s.Options)
		result = append(result, s)
	}

	return result, nil
}
