export interface Principal { id: string; kind: 'human' | 'bot'; name: string }
export type OrganisationRole = 'admin' | 'member'
export interface Organisation { id: string; name: string; role: OrganisationRole }
export interface Conversation { id: string; name: string; visibility: 'organisation' | 'members'; read_sequence?: number; latest_sequence?: number }
export interface Message {
  id: string; conversation_id: string; author_id: string; author_name: string
  author_kind: 'human' | 'bot'; body: string; client_id?: string; created_at: string; sequence: number
}
export interface User { id: string; kind: 'human' | 'bot'; name: string; role: OrganisationRole }
export interface EligibleUser { id: string; name: string; email: string }
