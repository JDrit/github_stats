package controllers

import (
    "database/sql"
    "github.com/coopernurse/gorp"
    "github.com/revel/revel/modules/db/app"
    _ "github.com/jbarham/gopgsqldriver" 
    r "github.com/revel/revel"
    "github_stats/app/models"
)

var (
    Dbm *gorp.DbMap
)

type GorpController struct {
    *r.Controller
    Txn *gorp.Transaction
}

func InitDB() {
    db.Init()
    Dbm = &gorp.DbMap{Db: db.Db, Dialect: gorp.PostgresDialect{}}

    Dbm.AddTableWithName(models.User{}, "users").SetKeys(false, "Login")
    Dbm.AddTableWithName(models.Repo{}, "repos").SetKeys(false, "Id")
}

func (c *GorpController) Begin() r.Result {
    txn, err := Dbm.Begin()
    if err != nil {
        panic(err)
    }
    c.Txn = txn
    return nil
}

func (c *GorpController) Commit() r.Result {
    if c.Txn == nil {
        return nil
    }
    if err := c.Txn.Commit(); err != nil && err != sql.ErrTxDone {
        panic(err)
    }
    c.Txn = nil
    return nil
}


func (c *GorpController) Rollback() r.Result {
    if c.Txn == nil {
        return nil
    }
    if err := c.Txn.Rollback(); err != nil && err != sql.ErrTxDone {
        panic(err)
    }
    c.Txn = nil
    return nil
}
