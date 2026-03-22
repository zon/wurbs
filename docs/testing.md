# Testing

## General Rules

Unit and integration tests should be written for features that don't create side effects outside of the app. Features that interact with the database or NATS may have integration tests but must only use test users and test channels.

Tests should never directly or indirectly write to the file system or Kubernetes cluster.

## Web Servers

The REST and socket web servers should have end-to-end tests written using only test users and test channels. End-to-end tests run against a local dev server, must only use public interfaces (e.g. HTTP endpoints, WebSocket connections), and must not connect directly to the database or NATS.

## wurbctl

Most of wurbctl's features create side effects. wurbctl should not have very much automated test coverage.

