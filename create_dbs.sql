drop table if exists users cascade;
drop table if exists files cascade;
drop table if exists repos cascade;

create table users (
    id integer unique,
    avatarUrl varchar(255),
    login varchar(255) primary key,
    name varchar(50),
    email varchar(255),
    followers integer,
    following integer,
    createdat bigint default 0,
    lastprocessed bigint default 0,
    reposleft int default 0
);

create table repos (
    id integer primary key,
    name varchar(255) not null,
    owner varchar(20) not null references users(login) on delete cascade on update cascade,
    description text,
    language varchar(255),
    stargazers integer default 0,
    forks integer default 0,
    createdAt bigint default 0
);

create table files (
    id serial primary key,
    name text not null,
    blank integer default 0,
    code integer default 0,
    comment integer default 0,
    language varchar(255),
    repoid integer not null references repos(id) on delete cascade on update cascade
);

create index file_language_idx on files(language);
create index repo_language_idx on repos(language);
create index repo_owner_idx on repos(owner);
create index user_login_idx on users(login);

alter table repos owner to github_stats;
alter table users owner to github_stats;
alter table files owner to github_stats;
