package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jackc/pgx/v5/stdlib"
	"time"
)

type DB struct{ SQL *sql.DB }

func NewDB(ctx context.Context, dsn string) (*DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	c, e := stdlib.ParseConfig(dsn)
	if e != nil {
		return nil, e
	}
	d := sql.OpenDB(stdlib.GetConnector(*c))
	d.SetMaxOpenConns(10)
	d.SetMaxIdleConns(5)
	d.SetConnMaxLifetime(30 * time.Minute)
	if e = d.PingContext(ctx); e != nil {
		d.Close()
		return nil, e
	}
	return &DB{d}, nil
}
func (d *DB) Close() error { return d.SQL.Close() }

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

func (d *DB) InsertQuestions(ctx context.Context, g, s string, sem, ch int, title, src string, qs []GeneratedQuestion) error {
	tx, e := d.SQL.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, q := range qs {
		if len(q.Options) != 4 {
			return fmt.Errorf("AI returned invalid options")
		}
		_, e = tx.ExecContext(ctx, `insert into soal(grade,subject,semester,chapter_no,chapter_title,question,option_a,option_b,option_c,option_d,correct_index,time_limit_ms,source_file) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, g, s, sem, ch, title, q.Question, q.Options[0], q.Options[1], q.Options[2], q.Options[3], q.CorrectIndex, q.TimeLimitMs, src)
		if e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (d *DB) GetQuestions(ctx context.Context, g, s string, sem, chap *int) ([]Question, error) {
	args := []any{g, s}
	where := "grade=$1 and subject=$2"
	n := 3
	if sem != nil {
		where += fmt.Sprintf(" and semester=$%d", n)
		args = append(args, *sem)
		n++
	}
	if chap != nil {
		where += fmt.Sprintf(" and chapter_no=$%d", n)
		args = append(args, *chap)
	}
	rows, e := d.SQL.QueryContext(ctx, `select id,grade,subject,semester,chapter_no,chapter_title,question,option_a,option_b,option_c,option_d,correct_index,time_limit_ms from soal where `+where+` order by chapter_no,id`, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []Question
	for rows.Next() {
		var q Question
		var a, b, c, dd string
		if e = rows.Scan(&q.ID, &q.Grade, &q.Subject, &q.Semester, &q.ChapterNo, &q.ChapterTitle, &q.Question, &a, &b, &c, &dd, &q.CorrectIndex, &q.TimeLimitMs); e != nil {
			return nil, e
		}
		q.Options = []string{a, b, c, dd}
		out = append(out, q)
	}
	return out, rows.Err()
}
func (d *DB) GetSetting(ctx context.Context, k string) (string, error) {
	var v string
	e := d.SQL.QueryRowContext(ctx, "select value from settings where key=$1", k).Scan(&v)
	return v, e
}
func (d *DB) SetSetting(ctx context.Context, k, v string) error {
	_, e := d.SQL.ExecContext(ctx, `insert into settings(key,value,updated_at) values($1,$2,now()) on conflict(key) do update set value=excluded.value,updated_at=now()`, k, v)
	return e
}

type Meta struct {
	Grades   []string `json:"grades"`
	Subjects []string `json:"subjects"`
}

func (d *DB) GetMeta(ctx context.Context) (Meta, error) {
	var m Meta
	rows, e := d.SQL.QueryContext(ctx, "select distinct grade from soal order by grade")
	if e != nil {
		return m, e
	}
	for rows.Next() {
		var x string
		rows.Scan(&x)
		m.Grades = append(m.Grades, x)
	}
	rows.Close()
	rows, e = d.SQL.QueryContext(ctx, "select distinct subject from soal order by subject")
	if e != nil {
		return m, e
	}
	for rows.Next() {
		var x string
		rows.Scan(&x)
		m.Subjects = append(m.Subjects, x)
	}
	rows.Close()
	return m, nil
}
