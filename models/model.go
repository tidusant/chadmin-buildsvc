package models

import (
	"time"

	"gopkg.in/mgo.v2/bson"
)

type User struct {
	ID     bson.ObjectId `bson:"_id,omitempty"`
	User   string        `bson:"user"`
	Name   string        `bson:"name"`
	Email  string        `bson:"email"`
	Active int32         `bson:"active"`
	Group  string        `bson:"group"`

	Config UserConfig `bson:"config"`
}

type UserConfig struct {
	Level     int `bson:"level"`
	MaxUpload int `bson:"maxupload"`
}

type Template struct {
	ID           bson.ObjectId `bson:"_id,omitempty"`
	Code         string        `bson:"code"`
	UserID       string        `bson:"userid"`
	Status       int           `bson:"status"` //-2: delete, -1: reject, 1: approved and publish, 2: pending, 3: approved but not publish
	Title        string        `bson:"title"`
	Description  string        `bson:"description"`
	Viewed       int           `bson:"viewed"`
	InstalledIDs []string      `bson:"installedid"`
	ActiveIDs    []string      `bson:"activedid"`
	Avatar       string        `bson:"avatar"`
	Created      time.Time     `bson:"created"`
	Modified     time.Time     `bson:"modified"`
	Content      string        `bson:"content"`
	CSS          string        `bson:"css"`
	Script       string        `bson:"script"`
	Images       string        `bson:"images"`
	Screenshot   string        `bson:"screenshot"`
	Configs      string        `bson:"configs"`
	Langs        string        `bson:"langs"`
}
