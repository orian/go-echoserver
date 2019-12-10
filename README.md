# go-echoserver
Simple Golang dockerized echo server

The purpose of this server is to allow easy debugging,
especially in the cloud. The server sets the HTTP server
on all specified addresses. The handler just prints all
the request information it gets, e.g.:

```
GET / HTTP/1.1
Host: localhost:12312
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
Accept-Encoding: gzip, deflate
Accept-Language: en-US,en;q=0.5
Connection: keep-alive
Dnt: 1
Upgrade-Insecure-Requests: 1
User-Agent: Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:70.0) Gecko/20100101 Firefox/70.0
```

## Start

Check [orian/echoserver](https://hub.docker.com/repository/docker/orian/echoserver) on DockerHub

```
docker run --rm -p 8080:8080 orian/echoserver
```

## Configuration

The configuration is done through environmental variables:

 - `LISTEN_ADDR` is a comma separated list of addresses: `localhost:8080,:8081`,
 default value is `:80,:8080`
 - `METRICS_PATH` exposes a Prometheus metrics for HTTP server, default is `/metrics`

If one wants to test the Postgres connection the config should be passed though:

 - `DB_ADDR` hostname of database
 - `DB_PORT` port the database listens at, default is `5432`
 - `DB_DATABASE` database name
 - `DB_USER` Postgres username
 - `DB_PASSWORD` a given user's password

SSL mode is configured through:
 - `DB_SSL_MODE` picks a sslmode, default is `disabled`, only if the mode is
 one of `allow,require,prefer,verify-ca,verify-full` the SSL will be used.
 - `DB_SSL_CERT` client certificate file.
 - `DB_SSL_KEY` client key file.
 - `DB_SSL_ROOT_CERT` server root certificate.
