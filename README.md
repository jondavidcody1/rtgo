rtgo
====

A Go real-time web framework.

### config.json
- **port** - the port to listen on
- **sslkey** - key file location; the file must contain PEM encoded data
- **sslcert** - cert file location; the file must contain PEM encoded data
- **hashkey** - the hash key for a [secure cookie](http://www.gorillatoolkit.org/pkg/securecookie#overview)
- **blockkey** - the block key for a [secure cookie](http://www.gorillatoolkit.org/pkg/securecookie#overview)
- **cookiename** - the name of the cookie to set
- **database** - the possible databases to use
  - **riak**
    - **host** - the host to connect to
    - **port** - the port to bind to
    - **tables** - a comma separated list of tables to use and/or create
  - **postgres**
    - **dbname** - the name of the database to connect to
    - **host** - the host to connect to
    - **user** - the user to sign in as
    - **password** - the user's password
    - **sslmode** - whether or not to use SSL (default is require, this is not the default for libpq)
    - **fallback_application_name** - an application_name to fall back to if one isn't provided
    - **connect_timeout** - maximum wait for connection, in seconds; zero or not specified means wait indefinitely
    - **sslcert** - cert file location; the file must contain PEM encoded data
    - **sslkey** - key file location; the file must contain PEM encoded data
    - **sslrootcert** - the location of the root certificate file; the file must contain PEM encoded data
    - **tables** - a comma separated list of tables to use and/or create
  - **mysql**
    - **host** - the host to connect to
    - **user** - the user to sign in as
    - **password** - the user's password
    - **dbname** - the name of the database to connect to
    - **allowAllFiles** - disables the file Whitelist for LOAD DATA LOCAL INFILE and allows all files
    - **allowCleartextPasswords** - allows using the cleartext client side plugin
    - **allowOldPasswords** - allows the usage of the insecure old password method
    - **charset** - sets the charset used for client-server interaction
    - **collation** - sets the collation used for client-server interaction on connection
    - **clientFoundRows** - causes an UPDATE to return the number of matching rows instead of the number of rows changed
    - **loc** - sets the location for time.Time values
    - **parseTime** - changes the output type of DATE and DATETIME values to time.Time instead of []byte / string
    - **strict** - enables the strict mode in which MySQL warnings are treated as errors
    - **timeout** - driver side connection timeout
    - **tls** - enables TLS / SSL encrypted connection to the server
    - **tables** - a comma separated list of tables to use and/or create
  - **sqlite3**
    - **file** - the path to the db file
    - **tables** - a comma separated list of tables to use and/or create
- **routes** - sets the possible routes
  - **path** - the path to match; if a regular expression must begin with '^' and end with '$'
    - **template** - the template to render
    - **table** - the table to query
    - **controllers** - a comma separated list of controllers associated to this path

### Command-line Tool
To install the command-line tool, enter the `rtgo/` subdirectory and run `go install`.  The following options are preceded by `rtgo`:
- **add** - add either a controller or a view
- **del** - delete either a controller or a view
- **controller** - follow this with the name of the controller to add or delete
- **view** - follow this with the name of the view to add or delete
- **create** - initialize a new rtgo application

### Example
```go
package main

import "github.com/gojonnygo/rtgo"

func main() {
    app := rtgo.NewApp()
    app.Emitter.On("event", func(conn *rtgo.Conn, data []byte, msg *rtgo.Message) {
        // do something here
    })
    app.Parse("./config.json")
    app.Start()
}
```
