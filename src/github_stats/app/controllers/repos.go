package controllers

import (
    "github.com/revel/revel"
    "github.com/revel/revel/cache"
    "github_stats/app/models"
    "github_stats/app/routes"
    "time"
)

type Repos struct {
    Application
}

func (c Repos) Stat() revel.Result {
    var results [][]int
    if err := cache.Get("stats", &results); err == nil {
        return c.RenderJson(results)
    }

    languages, _ := c.Txn.Select(models.RepoStat{},
        "select l.language from (select language from files where language != '' " + 
                "group by language order by count(*) desc limit 20) l order by l.language")
    results = make([][]int, 20)
    for i := 0 ; i < len(languages) ; i++ {
        revel.INFO.Printf(languages[i].(*models.RepoStat).Language)
        repoStats, _ := c.Txn.Select(models.RepoStat{}, 
            "select l.count, l.language from (select count(*) as count, language from files where repoid in " + 
                "(select id from repos where language = $1) and language in " + 
                "(select language from files where language != '' group by language order by count(*) desc limit 20) " + 
            "group by language order by count(*) desc) l order by l.language", languages[i].(*models.RepoStat).Language)
        row := make([]int, 20)
        for j := 0 ; j < len(repoStats) ; j++ {
            row[j] = repoStats[j].(*models.RepoStat).Count
            revel.INFO.Printf("\t\t%s %d\n", repoStats[j].(*models.RepoStat).Language, repoStats[j].(*models.RepoStat).Count)
        }
        results[i] = row
    }
    go cache.Set("stats", results, 1 * time.Hour)
    return c.RenderJson(results)
}

func (c Repos) Index() revel.Result {
	repoStats, err := c.Txn.Select(models.RepoStat{},
        "select count(*) from repos")
    if err != nil {
        panic(err)
    }

    fileStats, err := c.Txn.Select(models.FileStat{},
        "select count(*) from files")
    if err != nil {
        panic(err)
    }

    users, err := c.Txn.Select(models.UserStat{},
        "select count(distinct(owner)) from repos")

    repoCount := repoStats[0].(*models.RepoStat).Count
    userCount := users[0].(*models.UserStat).Count
    fileCount := fileStats[0].(*models.FileStat).Count
    return c.Render(repoCount, userCount, fileCount)
}

func (c Repos) Show(repoId int) revel.Result {
    repo, _ := c.Txn.Get(models.Repo{}, repoId)
    if repo == nil {
        c.Flash.Error("Repo does not exist")
        return c.Redirect(routes.Repos.Index())
    } else {
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

        return c.Render(files, repo, fileStats)
    }
}
