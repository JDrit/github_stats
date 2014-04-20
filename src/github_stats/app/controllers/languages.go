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

func (c Languages) Top() revel.Result {
    type Language struct {
        Name    string
        Color   string
    }
    var results []Language
    if err := cache.Get("languages", &results); err == nil {
        return c.RenderJson(results)
    }

    results = make([]Language, 20)
    languages, _ := c.Txn.Select(models.RepoStat{},
        "select l.language from (select language from files where language != '' " + 
                "group by language order by count(*) desc limit 20) l order by l.language")
    colors := [...]string {"#C0C0C0", "#808080", "#000000", 
        "#FF0000", "#800000", "#FFFF00", "#808000", "#00FF00",
        "#008000", "#00FFFF", "#008080", "#0000FF", "#000080", 
        "#FF00FF", "#800080", "#EEC591", "#458B00", "#FF7256", 
        "3F3FBF", "#8B0000"}
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
