package background

import (
    "github.com/revel/revel"
    "github.com/google/go-github/github"
    "code.google.com/p/goauth2/oauth"
    "github.com/streadway/amqp"
    "github_stats/app/models"
    "github.com/coopernurse/gorp"
    "github.com/revel/revel/modules/db/app"
    "time"
)

type AddUser struct {
    User models.User
}

/**
 * Adds all the user's repos the queue to process. This is done in the
 * background to allow for a faster page load for the user
 * s - the struct containing the model for the user
 */
func (s AddUser) Run() {
    var token string
    var found bool
    if token, found = revel.Config.String("api_token"); !found {
            revel.ERROR.Printf("no api token in the config")
            return
    } 
    dbm := &gorp.DbMap{Db: db.Db, 
        Dialect: gorp.PostgresDialect{}}
    dbm.AddTableWithName(models.User{}, "users").SetKeys(false, "Login")
    txn, err := dbm.Begin()
    if err != nil { panic(err) }

    t := &oauth.Transport{Token: &oauth.Token{AccessToken: token}} 
    client := github.NewClient(t.Client())

    /* gets all the repos for the user */
    page := 0
    var totalRepos []github.Repository
    for ; ; {
        options := github.RepositoryListOptions {
            ListOptions: github.ListOptions {
                Page: page,
                PerPage: 100,
            },
        }
        repos, _, _ := client.Repositories.List(s.User.Login, &options)
        for i := 0 ; i < len(repos) ; i++ {
            revel.INFO.Printf(*(repos[i].Name))
            totalRepos = append(totalRepos, repos[i])
        }
        page++
        revel.INFO.Printf("%d\n", page)
        if len(repos) != 100 {
            break
        }
    }
    /* sets the number of repos for the user */
    s.User.ReposLeft = len(totalRepos)
    txn.Update(&(s.User))
    txn.Commit()

    /* publishes each repo to the queue */
    spec, _ := revel.Config.String("amqp_url")
    conn, _ := amqp.Dial(spec)
    channel, _ := conn.Channel()
    for i := 0 ; i < len(totalRepos) ; i++ {
        repo := totalRepos[i]
        message := *(repo.Owner.Login) + "|" + *(repo.Name)
        msg := amqp.Publishing{
            DeliveryMode: amqp.Persistent,
            Timestamp:    time.Now(),
            ContentType:  "text/plain",
            Body:         []byte(message),
        }
        channel.Publish("", "repos-priority", false, false, msg) 
    }
}
