// Squirrel + Metago experiment: generated table/column names and scan helpers.
package main

//mgo:gen rawsql-tables

//mgo:model table=users
type User struct {
	ID     int64  //mgo:sql pk auto
	Name   string //mgo:sql
	Email  string //mgo:sql
	Age    int    //mgo:sql
	Active bool   //mgo:sql
	Score  float64
	Bio    *string
}
