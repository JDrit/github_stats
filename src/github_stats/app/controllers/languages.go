package controllers

import (
    "github.com/revel/revel"
    "github.com/revel/revel/cache"
    "github_stats/app/models"
    "github_stats/app/routes"
    "time"
)

type Languages struct {
    Application
}
// Used to get the list of top languages
func (c Languages) Top() revel.Result {
    type Language struct {
        Name    string
        Color   string
    }
    var results []Language
    limit := 20
    if err := cache.Get("languages", &results); err == nil {
        return c.RenderJson(results)
    }

    results = make([]Language, limit)
    languages, _ := c.Txn.Select(models.RepoStat{},
        "select l.language from (select language from files where language != '' " + 
                "group by language order by count(*) desc limit $1) l order by l.language", limit)
    colors := [...]string {"#FF0000", "#617C58", "#52D017", 
        "#C0C0C0", "#0000FF", "#808080", "#0000A0", "#ADD8E6",
        "#FFA500", "#800080", "#A52A2A", "#FFFF00", "#800000", 
        "#00FF00", "#008000", "#FF00FF", "#FF0000", "#57FEFF", 
        "FFA62F", "#8E35EF"}
    for i := 0 ; i < len(languages) ; i++ {
        results[i] = Language { 
            Name: languages[i].(*models.RepoStat).Language,
            Color: colors[i],
        }
    }
    go cache.Set("languages", results, 1 * time.Hour)
    return c.RenderJson(results)
}

func (c Languages) Index() revel.Result {
    var dbLineStats []interface{}
    var dbRepoStats []interface{}
    lines := 0
    err := cache.Get("dbLineStats", &dbLineStats)
    if err != nil {
        dbLineStats, _ = c.Txn.Select(models.FileStat{},
            "select language, sum(code + comment + blank) as lines " + 
            "from files group by language order by lines desc")
    }
    err = cache.Get("dbRepoStats", &dbRepoStats)
    if err != nil {
        dbRepoStats, _ = c.Txn.Select(models.RepoStat{},
            "select language, count(*) as count from repos where language != '' " + 
            "group by language order by count desc")
    }
    
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

    go cache.Set("dbLineStats", dbLineStats, 30 * time.Minute)
    go cache.Set("dbRepoStats", dbRepoStats, 30 * time.Minute)
    return c.Render(dbLineStats, lines, fileStats, repoStats)
}

func (c Languages) Show() revel.Result {
    language := c.Params.Get("language")
    if language == "" {
        return c.Redirect(routes.Languages.Index())
    }
    fileStats, err := c.Txn.Select(models.FileStat{}, 
        "select sum(code + comment + blank) as lines " + 
        "from files where language = $1", language)
    if err != nil {
        c.Flash.Error("Invalid language")
        return c.Redirect(routes.Languages.Index())
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
