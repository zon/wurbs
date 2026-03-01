# API Conversion

A plan to convert this repo to an API only chat service.

## Features

- REST JSON API
- Websocket messages
- Auth
  - Generic OIDC Auth
  - Client credentials flow
- Config file
- Secrets file
- Markdown documentation
- Support for multiple chat channels
  - Public channels
  - Private channels
  - Real channels
    - Can contain only real users
  - Test channels
    - Can contain both real and test users
- Admin users that can:
  - Manage channels
  - Invite users
  - Auth with both OIDC and client credential flow
- Users
  - Real
    - Can only auth with OIDC
  - Test
    - Can only auth with client credential flow
    - All share the same client credential flow keys
- NATS messages
  - Auth callout using k8s service tokens
    - Only the client side integration
- End to end tests
  - Requires admin client credential flow keys
  - Tests create and cleanup test channels and users
  - Tests avoid real channels, users, and OIDC
- Containerfile
  - Built with podman
  - Contains rest and socket services
  - Contains wurbctl binary
- Helm chart
  - Separate Deployment for the rest service
  - Separate Deployment for the socket service
  - Rest Deployment includes an init container that runs `wurbctl migrate db`
- make file
  - rest
    - starts rest service with air
  - socket
    - starts socket service with air
  - install
    - go install wurbctl
  - test
    - runs all tests
  - push
    - builds and pushes container
      - uses docker access token from file

## Services

- rest
  - REST JSON API service
  - Sends NATS messages matching record changes
- socket
  - HTTP API where clients can open Websocket connections following a channel
  - Relays NATS subject messages to Websocket channels

## Command Line

Create a wurbctl CLI app.

- wurbctl set config
  - create/updates postgres user and database
  - records OIDC settings
  - requires options providing anything missing the user must provide
    - db credentials
    - oidc settings
  - --test option to create/update test user client credential flow keys
    - deployments without test key can't create test users
  - creates/updates k8s configmap
  - creates/updates k8s secret
  - --local option to create configmap and secret files for local development
- wurbctl set admin
  - takes an email argument
  - creates/updates an admin user
  - generates admin client credential flow keys and saves to k8s secret
- wurbctl migrate db
  - runs database migrations against the configured postgres database
- Common options
  - --context defines what k8s to use
  - --namespace defines what k8s namespace to use

## Remove gonf dependency

Wurbs should no longer use the gonf library. All functionality currently provided by gonf must be implemented directly within this repo.

- Database
  - Replace `gonf.InitDB()` and `gonf.DB` with a locally-owned GORM setup
  - Replace `gonf.AutoMigrate()` with a local auto-migration call
- Auth
  - Replace `gonf.InitAuthMiddleware()` and `gonf.AuthMiddleware` with local OIDC and client credential middleware
  - Replace `gonf.AuthUser()` with a local helper that extracts the authenticated user from context
- NATS
  - Replace `gonf.Connect()` and `gonf.Publish()` with local NATS connection and publish helpers
- Config
  - Replace `gonf.LoadConfig()` with a local config loader reading from the config and secrets files
- Utilities
  - Replace `gonf.ParseTime()` with a local time parsing helper
  - Replace the `gonf.User` type with a locally-defined User model
- Remove the gonf replace directive from go.mod
- Remove client code (Vue/TS frontend that depends on gonf-ts)

## Clean up

- Remove create-db.sh

## Ralph Projects

This plan should be converted to two [Ralph](https://github.com/zon/ralph) projects.

- ./projects/bootstrap.yaml - Everything required to create Wurbs configmap and secret files for Ralph workflows
- ./projects/api.yaml - Everything else we need to do