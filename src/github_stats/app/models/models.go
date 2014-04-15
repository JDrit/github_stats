package models

import (
    "fmt"
)

type Repo struct {
    Id            int
    Name          string
    Owner         string
    Description   string
    Language      string
    Stargazers    int
    Forks         int
    CreatedAt     int64
}

type File struct {
    Id            int
    Name          string
    Blank         int
    Code          int
    Comment       int
    Language      string
    RepoId        int
}

type  FileStat struct {
    Language      string
    Lines         int
    Count         int
    Code          int
    Blank         int
    Comment       int
}

type UserStat struct {
    Count         int
}

type RepoStat struct {
    Language      string
    Count         int
}
type User struct {
    Id            int
    Name          string
    AvatarUrl     string
    Login         string
    Email         string
    Followers     int
    Following     int
    CreatedAt     int64
    LastProcessed int64
}

func (r *Repo) String() string { 
    return fmt.Sprintf("Repo: %s, owned by: %s", r.Name, r.Owner)
}

func (f *File) String() string {
    return fmt.Sprintf("File: %s", f.Name)
}

func (f *FileStat) String() string {
    return fmt.Sprintf("%s: %d", f.Language, f.Lines)
}

func (r *RepoStat) String() string {
    return fmt.Sprintf("%s: %d", r.Language, r.Count)
}
