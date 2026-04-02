
    const schema = {
  "asyncapi": "3.1.0",
  "info": {
    "title": "Wurbs",
    "version": "1.0.0",
    "description": "Chat application WebSocket API.\n\n**Authorization rules:**\n- Admins are implicit members of every channel\n- Only channel members may connect to a channel event stream\n"
  },
  "servers": {
    "websocket": {
      "host": "localhost:8081",
      "protocol": "ws",
      "description": "WebSocket event server",
      "security": [
        {
          "type": "openIdConnect",
          "openIdConnectUrl": "https://auth.example.com/.well-known/openid-configuration"
        }
      ]
    }
  },
  "channels": {
    "channelEvents": {
      "address": "/channels/{channelId}",
      "servers": [
        "$ref:$.servers.websocket"
      ],
      "description": "Real-time event stream for a channel. Only channel members may connect.",
      "parameters": {
        "channelId": {
          "description": "Channel identifier"
        }
      },
      "messages": {
        "MessageEvent": {
          "name": "MessageEvent",
          "summary": "Emitted when a message is created, edited, or deleted",
          "contentType": "application/json",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "message"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "created",
                  "updated",
                  "deleted"
                ],
                "x-parser-schema-id": "<anonymous-schema-2>"
              },
              "message": {
                "type": "object",
                "required": [
                  "id",
                  "channelId",
                  "userId",
                  "body",
                  "createdAt"
                ],
                "properties": {
                  "id": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-3>"
                  },
                  "channelId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-4>"
                  },
                  "userId": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-5>"
                  },
                  "body": {
                    "type": "string",
                    "description": "Markdown body",
                    "x-parser-schema-id": "<anonymous-schema-6>"
                  },
                  "createdAt": {
                    "type": "string",
                    "format": "date-time",
                    "x-parser-schema-id": "<anonymous-schema-7>"
                  },
                  "editedAt": {
                    "type": "string",
                    "format": "date-time",
                    "description": "Present if the message has been edited",
                    "x-parser-schema-id": "<anonymous-schema-8>"
                  }
                },
                "x-parser-schema-id": "Message"
              }
            },
            "x-parser-schema-id": "MessageEvent"
          },
          "x-parser-unique-object-id": "MessageEvent"
        },
        "MemberEvent": {
          "name": "MemberEvent",
          "summary": "Emitted when a user joins or leaves a channel",
          "contentType": "application/json",
          "payload": {
            "type": "object",
            "required": [
              "type",
              "user"
            ],
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "joined",
                  "left"
                ],
                "x-parser-schema-id": "<anonymous-schema-9>"
              },
              "user": {
                "type": "object",
                "required": [
                  "id",
                  "email",
                  "admin",
                  "inactive",
                  "createdAt"
                ],
                "properties": {
                  "id": {
                    "type": "string",
                    "x-parser-schema-id": "<anonymous-schema-10>"
                  },
                  "username": {
                    "type": "string",
                    "description": "Null until the user completes onboarding after first login",
                    "nullable": true,
                    "x-parser-schema-id": "<anonymous-schema-11>"
                  },
                  "email": {
                    "type": "string",
                    "format": "email",
                    "x-parser-schema-id": "<anonymous-schema-12>"
                  },
                  "admin": {
                    "type": "boolean",
                    "x-parser-schema-id": "<anonymous-schema-13>"
                  },
                  "inactive": {
                    "type": "boolean",
                    "x-parser-schema-id": "<anonymous-schema-14>"
                  },
                  "createdAt": {
                    "type": "string",
                    "format": "date-time",
                    "x-parser-schema-id": "<anonymous-schema-15>"
                  }
                },
                "x-parser-schema-id": "User"
              }
            },
            "x-parser-schema-id": "MemberEvent"
          },
          "x-parser-unique-object-id": "MemberEvent"
        },
        "UserEvent": {
          "name": "UserEvent",
          "summary": "Emitted when a channel member updates their username",
          "contentType": "application/json",
          "payload": {
            "type": "object",
            "required": [
              "userId",
              "username"
            ],
            "properties": {
              "userId": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-16>"
              },
              "username": {
                "type": "string",
                "x-parser-schema-id": "<anonymous-schema-17>"
              }
            },
            "x-parser-schema-id": "UserEvent"
          },
          "x-parser-unique-object-id": "UserEvent"
        },
        "Unsubscribe": {
          "name": "Unsubscribe",
          "summary": "Sent by the client to unsubscribe from the channel event stream.",
          "contentType": "application/json",
          "payload": {
            "type": "object",
            "x-parser-schema-id": "<anonymous-schema-18>"
          },
          "x-parser-unique-object-id": "Unsubscribe"
        }
      },
      "x-parser-unique-object-id": "channelEvents"
    }
  },
  "operations": {
    "subscribeChannelEvents": {
      "action": "receive",
      "summary": "Receive real-time events for a channel. Channel members only.",
      "channel": "$ref:$.channels.channelEvents",
      "messages": [
        "$ref:$.channels.channelEvents.messages.MessageEvent",
        "$ref:$.channels.channelEvents.messages.MemberEvent",
        "$ref:$.channels.channelEvents.messages.UserEvent"
      ],
      "x-parser-unique-object-id": "subscribeChannelEvents"
    },
    "unsubscribeChannelEvents": {
      "action": "send",
      "summary": "Unsubscribe from the channel event stream.",
      "channel": "$ref:$.channels.channelEvents",
      "messages": [
        "$ref:$.channels.channelEvents.messages.Unsubscribe"
      ],
      "x-parser-unique-object-id": "unsubscribeChannelEvents"
    }
  },
  "components": {
    "securitySchemes": {
      "oidc": "$ref:$.servers.websocket.security[0]"
    },
    "parameters": {
      "channelId": "$ref:$.channels.channelEvents.parameters.channelId"
    },
    "messages": {
      "Unsubscribe": "$ref:$.channels.channelEvents.messages.Unsubscribe",
      "MessageEvent": "$ref:$.channels.channelEvents.messages.MessageEvent",
      "MemberEvent": "$ref:$.channels.channelEvents.messages.MemberEvent",
      "UserEvent": "$ref:$.channels.channelEvents.messages.UserEvent"
    },
    "schemas": {
      "User": "$ref:$.channels.channelEvents.messages.MemberEvent.payload.properties.user",
      "Message": "$ref:$.channels.channelEvents.messages.MessageEvent.payload.properties.message",
      "MessageEvent": "$ref:$.channels.channelEvents.messages.MessageEvent.payload",
      "MemberEvent": "$ref:$.channels.channelEvents.messages.MemberEvent.payload",
      "UserEvent": "$ref:$.channels.channelEvents.messages.UserEvent.payload"
    }
  },
  "x-parser-spec-parsed": true,
  "x-parser-api-version": 3,
  "x-parser-spec-stringified": true
};
    const config = {"show":{"sidebar":true},"sidebar":{"showOperations":"byDefault"}};
    const appRoot = document.getElementById('root');
    AsyncApiStandalone.render(
        { schema, config, }, appRoot
    );
  