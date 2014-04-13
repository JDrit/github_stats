drop table if exists users cascade;
drop table if exists files cascade;
drop table if exists repos cascade;

create table users (
    id integer unique primary key,
    avatarUrl varchar(255),
    login varchar(255) unique,
    name varchar(255),
    email varchar(255),
    followers integer,
    following integer,
    createdat timestamp
);

create table repos (
    id integer unique primary key,
    name varchar(255) not null,
    owner varchar(255) not null references users(login) on update cascade on delete cascade,
    description text,
    language varchar(255),
    stargazers integer default 0,
    forks integer default 0
);

create table files (
    id serial primary key,
    name varchar(255) not null,
    blank integer default 0,
    code integer default 0,
    comment integer default 0,
    language varchar(255),
    repoid integer not null
);

create index file_language_idx on files(language);
create index repo_language_idx on repos(language);
create index repo_owner_idx on repos(owner);
create index user_login_idx on users(login);

alter table repos owner to github_stats;
alter table users owner to github_stats;
alter table files owner to github_stats;
