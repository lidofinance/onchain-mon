# Finding-forwarder
Service forwards findings from forta-local-node to telegram, opsGenie and discord

 ## How to develop
 1. Install go1.23.1+
 2. cd root repository
 3. make tools
 4. make vendor
 5. copy `sample.env` to `.env`
 6. clone https://github.com/lidofinance/alerting-forta/
    1. cd ethereum-steth
    2. make steth
 7. clone https://github.com/forta-network/forta-node/
    1. cd forta-node
    2. make containers
 8. Go back to current project then docker-compose up -d

### I dont need steth-bot. What I have to do?
1. comment service-ethereum-steth in docker-compose file
2. on your local host in your bot repo - ```yarn start```
3. Also, you have to provide the container name of your bot to FORTA-SOFTWARE by analogy with steth:
   1. Container name for steth is **ethereum-steth** look docker-compose file
   2. put bot's container name to the file forta-local-config.yml by analogy with steth
   3. Consider that in app has programmed consumer for your team bot or use fallback consumer
      (provider any name that not included in the list internal/utils/registry/registry.go)

### I want to develop finding-forwarder server or worker
1. comment forwarder-server or forwarder-worker in docker-compose file
2. provide env variable for application that it could connect to nats in docker

## Docs and rules
1. [App structure layout](./docs/structure.md)
2. [Code style](./docs/code_style.md)