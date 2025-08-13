import { get, post, getUser } from 'gonf'
import { reactive } from 'vue'
import type { Msg } from '@nats-io/nats-core'

export const messagesSubject = 'messages'

interface MessageData {
  ID: number
  UserID: number
  Content: string
  CreatedAt: string
  UpdatedAt: string
}

export class Message {
  id: number
  userId: number
  content: string
  createdAt: Date
  updatedAt: Date

  constructor(data: MessageData) {
    this.id = data.ID
    this.userId = data.UserID
    this.content = data.Content
    this.createdAt = new Date(data.CreatedAt)
    this.updatedAt = new Date(data.UpdatedAt)
  }

  getUser() {
    return getUser(this.userId)
  }

}

const path = 'messages'

export const messages = reactive<Message[]>([])

export async function onMessage(msg: Msg) {
  const data = msg.json<MessageData>()
  addMessage(new Message(data))
}

export function onMessageReconnect(disconnected: Date) {
  return updateMessages({after: disconnected})
}

export async function updateMessages(query?: {before?: Date, after?: Date}) {
  const list = await get<MessageData[]>(path, {
    before: query?.before?.toISOString() || '',
    after: query?.after?.toISOString() || ''
  })
  for (const data of list) {
    addMessage(new Message(data))
  }
}

export async function sendMessage(content: string) {
  const data = await post<MessageData>(path, content)
  return new Message(data)
}

function addMessage(message: Message) {
  for (let i = 0; i < messages.length; i++) {
    const other = messages[i]
    if (other.id === message.id) {
      messages[i] = message
      return
    }
    if (other.createdAt < message.createdAt) {
      messages.splice(i, 0, message)
      return
    }
  }
  messages.push(message)
}
