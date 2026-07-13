package sqlquery

//mgo:gen sql.exec in=Session.Token,Session.Data,Session.Expiry
const insertSessionSQL = `
	INSERT INTO sessions (token, data, expiry)
	VALUES (:token, :data, :expiry)
`

//mgo:gen sql.one in=Session.Token,Session.Expiry out=Session.Data
const findSessionSQL = `
	SELECT data
	FROM sessions
	WHERE token = :token
	  AND expiry > :expiry
`

//mgo:gen sql.one in=Session.Token out=Session scan=Session.Token,Session.Data,Session.Expiry
const getSessionSQL = `
	SELECT token, data, expiry
	FROM sessions
	WHERE token = :token
`

//mgo:gen sql.many in=Session.Expiry out=Session.Token
const listActiveSessionTokensSQL = `
	SELECT token
	FROM sessions
	WHERE expiry > :expiry
	ORDER BY token
`

//mgo:gen sql.many in=Session.Expiry out=Session scan=Session.Token,Session.Data,Session.Expiry
const listActiveSessionsSQL = `
	SELECT token, data, expiry
	FROM sessions
	WHERE expiry > :expiry
	ORDER BY token
`

//mgo:gen sql.exec in=Session.Token
const deleteSessionSQL = `
	DELETE FROM sessions
	WHERE token = :token
`
