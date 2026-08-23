export interface Principal { id: string; kind: 'human' | 'bot'; name: string }
export type OrganisationRole = 'admin' | 'member'
export interface Organisation { id: string; name: string; role: OrganisationRole }
export interface Conversation { id: string; name: string; visibility: 'organisation' | 'members'; member_ids?: string[]; read_sequence?: number; latest_sequence?: number; activity_at?: string; title_automatic?: boolean }
export interface MentionedUser { id: string; name: string }
export interface Attachment {
  id: string; media_type: 'image/jpeg' | 'image/png'; byte_size: number; width: number; height: number
  original_filename: string; created_at: string; content_url: string
}
export interface Message {
  id: string; conversation_id: string; author_id: string; author_name: string
  author_kind: 'human' | 'bot'; body: string; client_id?: string; created_at: string; sequence: number; mentions?: MentionedUser[]; attachments?: Attachment[]
}
export interface User { id: string; kind: 'human' | 'bot'; name: string; role: OrganisationRole }
export interface EligibleUser { id: string; name: string; email: string }
