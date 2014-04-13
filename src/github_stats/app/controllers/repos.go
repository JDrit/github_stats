package controllers

import (
    "github.com/revel/revel"
    "github_stats/app/models"
)

type Repos struct {
    Application
}

func (c Repos) Index() revel.Result {
	repoStats, err := c.Txn.Select(models.RepoStat{},
        "select count(*) from repos")
    if err != nil {
        panic(err)
    }

    users, err := c.Txn.Select(models.User{},
        "select count(distinct(owner)) from repos")
    if err != nil {
        panic(err)
    }

    fileStats, err := c.Txn.Select(models.FileStat{},
        "select count(*) from files")
    if err != nil {
        panic(err)
    }

    repoCount := repoStats[0].(*models.RepoStat).Count
    userCount := users[0].(*models.User).Count
    fileCount := fileStats[0].(*models.FileStat).Count
    return c.Render(repoCount, userCount, fileCount)
}

func (c Repos) Show(repoId int) revel.Result {
    revel.INFO.Printf("%d", repoId)
    files, err := c.Txn.Select(models.File{},
        "select * from files where repoid = $1", repoId)
    if err != nil {
        panic(err)
    }

    fileStats, err := c.Txn.Select(models.FileStat{}, 
        "select language, sum(code + comment + blank) as lines " + 
        "from files where repoid = $1 group by language", repoId)
    if err != nil {
        panic(err)
    }

    repos, err := c.Txn.Select(models.Repo{}, 
        "select * from repos where id = $1", repoId)
    if err != nil {
        panic(err)
    }
    repo := repos[0]
    return c.Render(files, repo, fileStats)
}
