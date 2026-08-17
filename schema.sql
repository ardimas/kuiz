create table if not exists settings (
 key text primary key,
 value text not null,
 updated_at timestamptz not null default now()
);
insert into settings(key,value) values ('site_passcode','123456') on conflict(key) do nothing;

create table if not exists soal (
 id bigserial primary key,
 grade text not null,
 subject text not null,
 semester smallint not null check (semester in (1,2)),
 chapter_no integer not null,
 chapter_title text not null,
 question text not null,
 option_a text not null,
 option_b text not null,
 option_c text not null,
 option_d text not null,
 correct_index smallint not null check (correct_index between 0 and 3),
 time_limit_ms integer not null default 15000 check (time_limit_ms between 1000 and 120000),
 source_file text,
 created_at timestamptz not null default now()
);
create index if not exists idx_soal_filter on soal(grade,subject,semester,chapter_no);
create index if not exists idx_soal_subject_semester on soal(subject,semester);
create index if not exists idx_soal_created_at on soal(created_at desc);
