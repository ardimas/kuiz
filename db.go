package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	SQL *sql.DB
}

func NewDB(ctx context.Context, dsn string) (*DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	d, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	d.SetMaxOpenConns(10)
	d.SetMaxIdleConns(5)
	d.SetConnMaxLifetime(30 * time.Minute)

	if err := d.PingContext(ctx); err != nil {
		d.Close()
		return nil, err
	}

	return &DB{SQL: d}, nil
}

func (d *DB) Close() error {
	return d.SQL.Close()
}

type Question struct {
	ID           int64    `json:"id"`
	Grade        string   `json:"grade"`
	Subject      string   `json:"subject"`
	Semester     int      `json:"semester"`
	ChapterNo    int      `json:"chapter_no"`
	ChapterTitle string   `json:"chapter_title"`
	Question     string   `json:"question"`
	Options      []string `json:"options"`
	CorrectIndex int      `json:"correct_index"`
	TimeLimitMs  int      `json:"time_limit_ms"`
}

type GeneratedQuestion struct {
	Question     string   `json:"question"`
	Options      []string `json:"options"`
	CorrectIndex int      `json:"correct_index"`
	TimeLimitMs  int      `json:"time_limit_ms"`
}

func (d *DB) InsertQuestions(
	ctx context.Context,
	g, s string,
	sem, ch int,
	title, src string,
	qs []GeneratedQuestion,
) error {
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, q := range qs {
		if len(q.Options) != 4 {
			return fmt.Errorf("AI returned invalid options")
		}

		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO soal
			(grade, subject, semester, chapter_no, chapter_title,
			 question, option_a, option_b, option_c, option_d,
			 correct_index, time_limit_ms, source_file)
			VALUES
			($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			g,
			s,
			sem,
			ch,
			title,
			q.Question,
			q.Options[0],
			q.Options[1],
			q.Options[2],
			q.Options[3],
			q.CorrectIndex,
			q.TimeLimitMs,
			src,
		)

		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) GetQuestions(
	ctx context.Context,
	g, s string,
	sem, chap *int,
) ([]Question, error) {
	args := []any{g, s}

	where := "grade=$1 AND subject=$2"
	n := 3

	if sem != nil {
		where += fmt.Sprintf(" AND semester=$%d", n)
		args = append(args, *sem)
		n++
	}

	if chap != nil {
		where += fmt.Sprintf(" AND chapter_no=$%d", n)
		args = append(args, *chap)
	}

	rows, err := d.SQL.QueryContext(
		ctx,
		`SELECT
			id,
			grade,
			subject,
			semester,
			chapter_no,
			chapter_title,
			question,
			option_a,
			option_b,
			option_c,
			option_d,
			correct_index,
			time_limit_ms
		FROM soal
		WHERE `+where+`
		ORDER BY chapter_no, id`,
		args...,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Question

	for rows.Next() {
		var q Question
		var a, b, c, dd string

		if err := rows.Scan(
			&q.ID,
			&q.Grade,
			&q.Subject,
			&q.Semester,
			&q.ChapterNo,
			&q.ChapterTitle,
			&q.Question,
			&a,
			&b,
			&c,
			&dd,
			&q.CorrectIndex,
			&q.TimeLimitMs,
		); err != nil {
			return nil, err
		}

		q.Options = []string{a, b, c, dd}
		out = append(out, q)
	}

	return out, rows.Err()
}

func (d *DB) GetSetting(ctx context.Context, k string) (string, error) {
	var v string

	err := d.SQL.QueryRowContext(
		ctx,
		"SELECT value FROM settings WHERE key=$1",
		k,
	).Scan(&v)

	return v, err
}

func (d *DB) SetSetting(ctx context.Context, k, v string) error {
	_, err := d.SQL.ExecContext(
		ctx,
		`INSERT INTO settings(key,value,updated_at)
		 VALUES($1,$2,now())
		 ON CONFLICT(key)
		 DO UPDATE SET
			value=excluded.value,
			updated_at=now()`,
		k,
		v,
	)

	return err
}

type Meta struct {
	Grades   []string `json:"grades"`
	Subjects []string `json:"subjects"`
}

func (d *DB) GetMeta(ctx context.Context) (Meta, error) {
	var m Meta

	rows, err := d.SQL.QueryContext(
		ctx,
		"SELECT DISTINCT grade FROM soal ORDER BY grade",
	)
	if err != nil {
		return m, err
	}

	for rows.Next() {
		var x string

		if err := rows.Scan(&x); err != nil {
			rows.Close()
			return m, err
		}

		m.Grades = append(m.Grades, x)
	}

	rows.Close()

	rows, err = d.SQL.QueryContext(
		ctx,
		"SELECT DISTINCT subject FROM soal ORDER BY subject",
	)
	if err != nil {
		return m, err
	}

	for rows.Next() {
		var x string

		if err := rows.Scan(&x); err != nil {
			rows.Close()
			return m, err
		}

		m.Subjects = append(m.Subjects, x)
	}

	rows.Close()

	return m, nil
}
