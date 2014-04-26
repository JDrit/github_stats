package controllers

import (
    "github.com/revel/revel"
    "github_stats/app/models"
    "github.com/revel/revel/modules/jobs/app/jobs"
    "github_stats/app/routes"
    "github_stats/app/background"
    "github.com/google/go-github/github"
    "code.google.com/p/goauth2/oauth"
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
    users, _ := c.Txn.Select(models.User{}, 
        "select * from users where lower(login) = lower($1)", 
        login)
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
            if user.Name != nil { name = *(user.Name) }
            if user.Email != nil { email = *(user.Email) }
            newUser := models.User{
                Id: *(user.ID), 
                Name: name, 
                Login: *(user.Login), 
                AvatarUrl: *(user.AvatarURL),
                Email: email, 
                Followers: *(user.Followers), 
                Following: *(user.Following), 
                //ReposLeft: len(totalRepos),
                CreatedAt: (*(user.CreatedAt)).Unix()}
            c.Txn.Insert(&newUser)
            c.Flash.Error("User not found. User has been added to queue to process. Come back shortly!")
            jobs.Now(background.AddUser{User: newUser})
        }
        return c.Redirect(routes.Users.Show(login))
    } else {
        user := users[0]
        repos,e := c.Txn.Select(models.Repo{}, 
            "select * from repos where owner = $1", user.(*models.User).Login)
        if e != nil {
            revel.INFO.Printf(e.Error())
        }
        repoCount := len(repos)
        working := user.(*models.User).LastProcessed == 0
        if repoCount > 0 {
            fileStats, _ := c.Txn.Select(models.FileStat{}, "select language, sum(code + comment + blank) as lines " + 
                "from files where repoid in (select id from repos where owner = $1) group by language " + 
                "order by lines desc limit 10", user.(*models.User).Login)
            return c.Render(repos, fileStats, login, repoCount, user, working)
        } else {
            return c.Render(repos, login, repoCount, user, working)
        }
    }
}
