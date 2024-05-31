# Finding-forwarder
Service forwards findings from forta-local-node to telegram, opsGenia and discord

 ## How to use the template
 1. Clone repository
 2. cd root repository
 3. make tools
 4. make vendor
 5. copy `sample.env` to `.env` 
 6. docker-compose up -d
 7. make build
 8. Run service ./bin/service

## Where I have to start to code my custom logic?
* [Register handler](./internal/app/server/routes.go)
* [Logic layer](./internal/pkg/telegram): /internal/pkg/your_package_name/. Just see an example with [User package](./internal/pkg/telegram)
* [Env](./internal/env/env.go)
* [Connecters](./internal/connectors) pg, logger, redis and etc...
* For external clients you have to create folder in ./internal/clients/<your_client_name>/client.go where your_client_name - is google_client, alchemy or internal client for private network.

## Docs and rules
1. [App structure layout](./docs/structure.md)
2. [Code style](./docs/code_style.md)

## Current drivers or dependencies

1. Logger - [Logrus](https://github.com/sirupsen/logrus)
2. Mockery [Mockery](https://github.com/vektra/mockery)
3. Http router [go-chi](https://github.com/go-chi/chi)
4. Env reader [Viper](https://github.com/spf13/viper)
