package controllers

import "github.com/revel/revel"

type Users struct {
    Application
}

func (c Users) Index() revel.Result {
    message := "testing 1 2 3"
	return c.Render(message)
}

func (c Users) Show() revel.Result {
    return c.Render()
}
