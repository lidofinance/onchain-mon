# go-template
go-skeleton-lido

 ## How to use the template
 1. Clone repository
 2. cd root repository
 3. make tools
 4. make vendor
 5. copy `sample.env` to `.env` 
 6. docker-compose up -d
 7. make migrate
 8. make build
 9. Run service ./bin/service

## How to create migrations?
 ./bin/migrate create -ext=sql -dir=db/migrations <your table name>

## How to make migrations?
1. make migrate from terminal or 
```
    bin/migrate -database ${POSTGRESQL_URL} -path db/migrations up
```

## Where I have to start to code my custom logic?
* [Register handler](./internal/app/server/routes.go)
* [Logic layer](./internal/pkg/users): /internal/pkg/your_package_name/. Just see an example with [User package](./internal/pkg/users)
* [Env](./internal/env/env.go)
* [Connecters](./internal/connectors) pg, logger, redis and etc...
* For external clients you have to create folder in ./internal/clients/<your_client_name>/client.go where your_client_name - is google_client, alchemy or internal client for private network.

## Docs and rules
1. [App structure layout](./docs/structure.md)
2. [Code style](./docs/code_style.md)

## Current drivers or dependencies
1. Postgres - [pgx](https://github.com/jackc/pgx)
2. Logger - [Logrus](https://github.com/sirupsen/logrus)
3. Mockery [Mockery](https://github.com/vektra/mockery)
4. Http router [gorilla_mux](github.com/gorilla/mux). Of course your can change it for example to [Gin](https://github.com/gin-gonic/gin)
5. Env reader [Viper](https://github.com/spf13/viper)
