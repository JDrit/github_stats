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
    CreatedAt     string
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
}

type RepoStat struct {
    Language      string
    Count         int
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
