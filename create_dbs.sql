drop table if exists files;
drop table if exists repos;

create table repos (
    id integer unique not null,
    name varchar(255) not null,
    owner varchar(255) not null,
    description varchar(255),
    language varchar(255),
    createdat timestamp
);

create table files (
    id serial,
    name varchar(255) not null,
    blank integer default 0,
    code integer default 0,
    comment integer default 0,
    language varchar(255),
    repoid integer
);

alter table repos owner to github_stats;
alter table files owner to github_stats;
