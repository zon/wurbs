package core

import "github.com/zon/chat/core/message"

// Message is a type alias for message.Message. The message module owns the
// Message model; this alias keeps existing references to core.Message working.
type Message = message.Message
