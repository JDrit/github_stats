package controllers

import (
    "github.com/revel/revel"
    "github_stats/app/models"
)

type Languages struct {
    Application
}

func (c Languages) Index() revel.Result {
    dbLineStats, _ := c.Txn.Select(models.FileStat{},
        "select language, sum(code + comment + blank) as lines " + 
        "from files group by language order by lines desc")
    dbRepoStats, _ := c.Txn.Select(models.RepoStat{},
        "select language, count(*) as count from repos where language != '' " + 
        "group by language order by count desc")
    lines := 0
    
    fileStats := make([](*models.FileStat), 15)
    for i := 0 ; i < len(dbLineStats) ; i++ {
        f := dbLineStats[i].(*models.FileStat)
        lines += f.Lines
        if i < 15 {
            fileStats[i] = dbLineStats[i].(*models.FileStat)
        }
        dbLineStats[i].(*models.FileStat).Count = i + 1
    }
    repoStats := make([](*models.RepoStat), 15)
    for i := 0 ; i < len(repoStats) ; i++  {
        repoStats[i] = dbRepoStats[i].(*models.RepoStat)
    }
    return c.Render(dbLineStats, lines, fileStats, repoStats)
}

func (c Languages) Show() revel.Result {
    language := c.Params.Get("language")
    fileStats, err := c.Txn.Select(models.FileStat{}, 
        "select sum(code + comment + blank) as lines " + 
        "from files where language = $1", language)
    if err != nil {
        panic(err)
    }

    lineStats, _ := c.Txn.Select(models.FileStat{},
        "select sum(code) as code, sum(comment) as comment, " + 
        "sum(blank) as blank from files where language = $1", language)
    if err != nil {
        panic(err)
    }

    repoStats, err := c.Txn.Select(models.RepoStat{},
        "select count(*) from repos " + 
        "where language = $1", language)
    if err != nil {
        panic(err)
    }
    
    fileStatsCount, err := c.Txn.Select(models.FileStat{},
        "select count(*) from files where language = $1", language)
    if err != nil {
        panic(err)
    }

    repos, err := c.Txn.Select(models.Repo{}, 
        "select * from repos where language = $1", 
        language)
    if err != nil {
        panic(err)
    }


    lines := fileStats[0].(*models.FileStat).Lines
    repoCount := repoStats[0].(*models.RepoStat).Count
    fileCount := fileStatsCount[0].(*models.FileStat).Count
    blank := lineStats[0].(*models.FileStat).Blank
    code := lineStats[0].(*models.FileStat).Code
    comment := lineStats[0].(*models.FileStat).Comment
    return c.Render(language, lines, repoCount, repos, fileCount, 
        blank, code, comment)
}
