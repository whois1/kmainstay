export interface Principal { id: string; kind: 'human' | 'bot'; name: string }
export interface Organisation { id: string; name: string }
export interface Conversation { id: string; name: string; visibility: 'organisation' | 'members' }
export interface Message {
  id: string; conversation_id: string; author_id: string; author_name: string
  author_kind: 'human' | 'bot'; body: string; client_id?: string; created_at: string; sequence: number
}
export interface User { id: string; kind: 'human' | 'bot'; name: string }
