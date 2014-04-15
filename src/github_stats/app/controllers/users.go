package controllers

import (
    "github.com/revel/revel"
    "github_stats/app/models"
    "github_stats/app/routes"
    "github_stats/app/background"
    "github.com/revel/revel/modules/jobs/app/jobs"
    "github.com/google/go-github/github"
    "code.google.com/p/goauth2/oauth"
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

func (c Users) Search() revel.Result {
    username := c.Params.Get("username")
    return c.Redirect(routes.Users.Show(username))
}


func (c Users) Show(login string) revel.Result {
    revel.INFO.Printf(login)
    users, e := c.Txn.Select(models.User{}, 
        "select * from users where lower(login) = lower($1)", 
        login)
    if e != nil {
        revel.INFO.Printf(e.Error())
    }
    if len(users) == 0 {
        if token, found := revel.Config.String("api_token"); !found {
            revel.ERROR.Printf("no api token in the config")
        } else {
            t := &oauth.Transport{ Token: &oauth.Token{AccessToken: token} } 
            client := github.NewClient(t.Client())

            user, _, err := client.Users.Get(login)

            if err != nil {
                revel.ERROR.Printf("err getting user %s\n", err.Error())
                c.Flash.Error("Could not find user, are you sure the login name is correct?")
                return c.Redirect(routes.Users.Index())
            }
            if user == nil {
                revel.ERROR.Printf("user does not exist %s\n", login)
                c.Flash.Error("Could not find user, are you sure the login name is correct?")
                return c.Redirect(routes.Users.Index())
            }
            name := ""
            email := ""
            if user.Name != nil {
                name = *(user.Name)
            }
            if user.Email != nil {
                email = *(user.Email)
            }
            newUser := models.User{
                Id: *(user.ID), 
                Name: name, 
                Login: *(user.Login), 
                AvatarUrl: *(user.AvatarURL),
                Email: email, 
                Followers: *(user.Followers), 
                Following: *(user.Following), 
                CreatedAt: (*(user.CreatedAt)).Unix()}
            err = c.Txn.Insert(&newUser)
            if err != nil {
                revel.ERROR.Printf("error adding user, %s", err.Error())
            }
            c.Flash.Error("User not found. User has been added to queue to process. Come back shortly!")
            job := background.ProcessUser{login}
            jobs.Now(job)
        }
        return c.Redirect(routes.Users.Show(login))
    } else {
        user := users[0]
        repos, _ := c.Txn.Select(models.Repo{}, 
            "select * from repos where lower(owner) = lower($1)", login)
        repoCount := len(repos)
        working := user.(*models.User).LastProcessed == 0
        if repoCount > 0 {
            var repoIds bytes.Buffer
            repoIds.WriteString("select language, sum(code + comment + blank) as lines " + 
                "from files where repoid in (")
            for i := 0 ; i < len(repos) - 1; i++ {
                repoIds.WriteString(strconv.Itoa(repos[i].(*models.Repo).Id) + ", ")
            }
            repoIds.WriteString(strconv.Itoa(repos[len(repos) - 1].(*models.Repo).Id))
            repoIds.WriteString(") group by language order by lines desc limit 10")
            
            fileStats, _ := c.Txn.Select(models.FileStat{}, repoIds.String())
            return c.Render(repos, fileStats, login, repoCount, user, working)
        } else {
            return c.Render(repos, login, repoCount, user, working)
        }
    }
}
