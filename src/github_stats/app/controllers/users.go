package controllers

import (
    "github.com/revel/revel"
    "github_stats/app/models"
    "bytes"
    "strconv"
)

type Users struct {
    Application
}

func (c Users) Index() revel.Result {
    message := "testing 1 2 3"
	return c.Render(message)
}

func (c Users) Show(login string) revel.Result {
    users, _ := c.Txn.Select(models.User{}, 
        "select * from users where login = $1", login) 
    user := users[0]
    repos, _ := c.Txn.Select(models.Repo{}, 
        "select * from repos where owner = $1", login)
    repoCount := len(repos)
    var repoIds bytes.Buffer
    repoIds.WriteString("select language, sum(code + comment + blank) as lines " + 
        "from files where repoid in (")
    for i := 0 ; i < len(repos) - 1; i++ {
        repoIds.WriteString(strconv.Itoa(repos[i].(*models.Repo).Id) + ", ")
    }
    repoIds.WriteString(strconv.Itoa(repos[len(repos) - 1].(*models.Repo).Id))
    repoIds.WriteString(") group by language order by lines desc limit 10")
    
    fileStats, _ := c.Txn.Select(models.FileStat{}, repoIds.String())
    return c.Render(repos, fileStats, login, repoCount, user)
}
