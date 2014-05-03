package controllers

import (
    "github.com/revel/revel"
    "github.com/revel/revel/cache"
    "github_stats/app/models"
    "github_stats/app/routes"
    "time"
    "bytes"
)

type Repos struct {
    Application
}

type Language struct {
    Name    string
    Color   string
}

func (c Repos) Stat() revel.Result {
    limit := 15
    var results [][]int
    var buffer bytes.Buffer
    var mapResults map[string]interface{}
    if err := cache.Get("stats", &mapResults); err == nil {
        return c.RenderJson(mapResults)
    }
    languageResults := make([]Language, limit)
    languages, _ := c.Txn.Select(models.RepoStat{},
        "select l.language from (select language from files where language != '' " + 
                "group by language order by sum(code + comment + blank) desc limit $1) " + 
                "l order by l.language", limit)
    colors := [...]string {"#FF0000", "#617C58", "#52D017", 
        "#C0C0C0", "#0000FF", "#808080", "#0000A0", "#ADD8E6",
        "#FFA500", "#800080", "#A52A2A", "#FFFF00", "#800000", 
        "#00FF00", "#008000", "#FF00FF", "#FF0000", "#57FEFF", 
        "FFA62F", "#8E35EF"}
    for i := 0 ; i < len(languages) ; i++ {
        languageResults[i] = Language { 
            Name: languages[i].(*models.RepoStat).Language,
            Color: colors[i],
        }
        buffer.WriteString("'" + languages[i].(*models.RepoStat).Language + "', ")
    }
    buffer.WriteString("'" + languages[len(languages) - 1].(*models.RepoStat).Language + "'")
    mapResults = make(map[string]interface{})
    mapResults["languages"] = languageResults
    results = make([][]int, limit)
    for i := 0 ; i < len(languages) ; i++ {
        repoStats, _ := c.Txn.Select(models.RepoStat{}, 
            "select l.count, l.language from (select sum(code + comment + blank) as count, language " + 
            "from files where repoid in (select id from repos where language = $1) " + 
            "and language in (" + buffer.String()  + ") group by language order by " + 
            "sum(code + comment + blank) desc) l order by l.language", 
            languages[i].(*models.RepoStat).Language)
        row := make([]int, limit)
        for j := 0 ; j < len(repoStats) ; j++ {
            var index int
            for k := 0 ; k < limit ; k++ {
                if languageResults[k].Name == repoStats[j].(*models.RepoStat).Language {
                    index = k
                    break
                }
            }
            row[index] = repoStats[j].(*models.RepoStat).Count
        }
        results[i] = row
    }
    mapResults["stats"] = results
    go cache.Set("stats", mapResults, 1 * time.Hour)
    return c.RenderJson(mapResults)
}

func (c Repos) Index() revel.Result {
    var repoCount int64
    var fileCount int64
    var userCount int64
    var speed int
    if err := cache.Get("repoCount", &repoCount); err != nil {
        repoCount, _ = c.Txn.SelectInt("select count(*) from repos")
        go cache.Set("repoCount", repoCount, 1 * time.Hour)
    }
    if err := cache.Get("fileCount", &fileCount); err != nil {
        fileCount, _ = c.Txn.SelectInt("select count(*) from files")
        go cache.Set("fileCount", fileCount, 1 * time.Hour)
    }
    if err := cache.Get("userCount", &userCount); err != nil {
        userCount, _ = c.Txn.SelectInt("select count(distinct(owner)) from repos")
        go cache.Set("userCount", userCount, 1 * time.Hour)
    }
    cache.Get("speed", &speed) // lines processed per second
    return c.Render(repoCount, userCount, fileCount, speed)
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
