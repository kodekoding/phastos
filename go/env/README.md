[![godoc](https://godoc.org/github.com/golang/mock/gomock?status.svg)](http://godoc.tkpd/pkg/github.com/kodekoding/phastos/v2/go/env)

# Env
This small package give some functionality to common usage of detecting Tokopedia runtime ecosystem using `TKPENV` environment variable.

Other usecase is to set the environment variables using file, this is handy if you want to run your app in your laptop with custom value depending on your local ecosystem. 
For example your app will read`DB_CONN` to fetch connection string to database, you can create file containing envar to set before application initialize connection to database.

At default `env` will look for file `.env` in current directory and load the value. If you want to load env file from other place then you can use `SetFromEnvFile(<path>)`.

## Example
**.env**
```
DB_CONN=postgres://foo@bar/database
```

**main.go**
```go
import _ "github.com/kodekoding/phastos/v2/go/env" // this will enough just to trigger env to look for .env file

func main(){
    initializeDbConnection() // your function that read DB_CONN 
}
```
