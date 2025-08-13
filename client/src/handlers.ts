import { messagesSubject, onMessage, onMessageReconnect } from './models/Message'
import { usersSubject, onUser, onUserReconnect, natsHandlers } from 'gonf'

natsHandlers.push(
  {subject: messagesSubject, onMsg: onMessage, onReconnect: onMessageReconnect},
  {subject: usersSubject, onMsg: onUser, onReconnect: onUserReconnect}
)
