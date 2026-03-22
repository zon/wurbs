# Testing

## General Rules

Unit and integration tests should be written for features that don't create side effects outside of the app. Features that interact with the database or NATS may have integration tests but must only use test users and test channels.

Tests should never directly or indirectly write to the file system or Kubernetes cluster.

## Web Servers

The REST and socket web servers should have end-to-end tests written using only test users and test channels. End-to-end tests assume the REST and socket servers are already running via `air` in the background. Tests must only use public interfaces (e.g. HTTP endpoints, WebSocket connections), and must not connect directly to the database or NATS. End-to-end tests should not be written for authentication endpoints (e.g. /auth/login, /auth/callback, /auth/logout, /auth/refresh), but every other endpoint in the REST spec must have at least one end-to-end test.

## wurbctl

Most of wurbctl's features create side effects. wurbctl should not have very much automated test coverage.

