// Squirrel + Metago experiment: generated table/column names and scan helpers.
package main

//mgo:gen tables

//mgo:props model table=users
type User struct {
	ID     int64  //mgo:props sql pk auto
	Name   string //mgo:props sql
	Email  string //mgo:props sql
	Age    int    //mgo:props sql
	Active bool   //mgo:props sql
	Score  float64
	Bio    *string
}
