package controllers

import (
    "github.com/revel/revel"
    "github_stats/app/models"
)

type Languages struct {
    Application
}

func (c Languages) Index() revel.Result {
    fileStats, err := c.Txn.Select(models.FileStat{},
        "select language, sum(code + comment + blank) as lines " + 
        "from files group by language order by lines desc")
    lines := 0
    if err != nil {
        panic(err)
    }
    i := 0
    stats := make([](*models.FileStat), 15)
    for _, r := range fileStats {
        f := r.(*models.FileStat)
        lines += f.Lines
        if i < 15 {
            stats[i] = r.(*models.FileStat)
            i += 1
        }
    }
    return c.Render(stats, lines, fileStats)
}

func (c Languages) Show(language string) revel.Result {
    fileStats, err := c.Txn.Select(models.FileStat{}, 
        "select sum(code + comment + blank) as lines " + 
        "from files where language = $1", language)
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
    return c.Render(language, lines, repoCount, repos, fileCount)
}
